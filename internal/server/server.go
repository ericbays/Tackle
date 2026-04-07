// Package server sets up the HTTP server, router, and middleware chain.
package server

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"tackle/internal/compiler"
	"tackle/internal/compiler/hosting"
	"tackle/internal/config"
	"tackle/internal/crypto"
	"tackle/internal/endpoint"
	"tackle/internal/endpoint/cloud"
	"tackle/internal/handlers"
	aihandlers "tackle/internal/handlers/ai"
	apikeyhandlers "tackle/internal/handlers/apikeys"
	audithandlers "tackle/internal/handlers/audit"
	authhandlers "tackle/internal/handlers/auth"
	authprovhandlers "tackle/internal/handlers/authproviders"
	campaignhandlers "tackle/internal/handlers/campaigns"
	cloudhandlers "tackle/internal/handlers/cloudcredentials"
	credhandlers "tackle/internal/handlers/credentials"
	dnshandlers "tackle/internal/handlers/dns"
	dphandlers "tackle/internal/handlers/domainproviders"
	domainhandlers "tackle/internal/handlers/domains"
	emaildeliveryhandlers "tackle/internal/handlers/emaildelivery"
	ethandlers "tackle/internal/handlers/emailtemplates"
	endpointhandlers "tackle/internal/handlers/endpoints"
	healthhandlers "tackle/internal/handlers/health"
	internalhandlers "tackle/internal/handlers/internalapi"
	lphandlers "tackle/internal/handlers/landingpages"
	metricshandlers "tackle/internal/handlers/metrics"
	notifhandlers "tackle/internal/handlers/notification"
	permhandlers "tackle/internal/handlers/permissions"
	reporthandlers "tackle/internal/handlers/reports"
	rolehandlers "tackle/internal/handlers/roles"
	searchhandlers "tackle/internal/handlers/search"
	settingshandlers "tackle/internal/handlers/settings"
	setuphandlers "tackle/internal/handlers/setup"
	smtphandlers "tackle/internal/handlers/smtp"
	tghandlers "tackle/internal/handlers/targetgroups"
	targethandlers "tackle/internal/handlers/targets"
	userhandlers "tackle/internal/handlers/users"
	"tackle/internal/middleware"
	"tackle/internal/providers/credentials"
	"tackle/internal/repositories"
	apikeysvc "tackle/internal/services/apikey"
	auditsvc "tackle/internal/services/audit"
	authsvc "tackle/internal/services/auth"
	authprovsvc "tackle/internal/services/authprovider"
	blocklistsvc "tackle/internal/services/blocklist"
	campaignsvc "tackle/internal/services/campaign"
	catsvc "tackle/internal/services/categorization"
	cloudsvc "tackle/internal/services/cloudcredential"
	credsvc "tackle/internal/services/credential"
	dnssvc "tackle/internal/services/dns"
	domainsvc "tackle/internal/services/domain"
	dpsvc "tackle/internal/services/domainprovider"
	emaildeliverysvc "tackle/internal/services/emaildelivery"
	emailtmplsvc "tackle/internal/services/emailtemplate"
	endpointmgmtsvc "tackle/internal/services/endpointmgmt"
	healthsvc "tackle/internal/services/health"
	tmplsvc "tackle/internal/services/instancetemplate"
	lpsvc "tackle/internal/services/landingpage"
	metricssvc "tackle/internal/services/metrics"
	notifsvc "tackle/internal/services/notification"
	reportsvc "tackle/internal/services/report"
	smtpsvc "tackle/internal/services/smtpprofile"
	targetsvc "tackle/internal/services/target"
	targetgroupsvc "tackle/internal/services/targetgroup"
	typosvc "tackle/internal/services/typosquat"
	"tackle/internal/tracking"
	"tackle/internal/workers"
)

const (
	shutdownTimeout = 30 * time.Second
	buildVersion    = "dev"

	// Request body size limits (REQ-API-018).
	bodySizeStandard = 1 << 20 // 1 MB
	bodySizeBatch    = 5 << 20 // 5 MB
)

// New creates and returns a configured HTTP server and the audit service for system event logging.
func New(cfg *config.Config, db *sql.DB, masterKey []byte, logger *slog.Logger) (*http.Server, *auditsvc.AuditService) {
	r, auditService := buildRouter(cfg, db, masterKey, logger)
	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if cfg.TLSEnabled {
		srv.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			},
		}
	}

	srv.RegisterOnShutdown(func() {
		auditService.Drain()
	})
	return srv, auditService
}

// Run starts the server and blocks until a shutdown signal is received.
// It performs a graceful shutdown with a 30-second timeout.
// When cfg.TLSEnabled is true, the server starts with TLS and an HTTP→HTTPS
// redirect server is launched on port 80.
func Run(srv *http.Server, cfg *config.Config, logger *slog.Logger) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	serverErr := make(chan error, 1)

	if cfg.TLSEnabled {
		// Start HTTP→HTTPS redirect server on port 80.
		redirectSrv := &http.Server{
			Addr:         ":80",
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				target := "https://" + r.Host + r.URL.RequestURI()
				http.Redirect(w, r, target, http.StatusMovedPermanently)
			}),
		}
		go func() {
			logger.Info("HTTP→HTTPS redirect server starting", slog.String("addr", ":80"))
			if err := redirectSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("redirect server failed", "error", err)
			}
		}()
		srv.RegisterOnShutdown(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = redirectSrv.Shutdown(ctx)
		})

		go func() {
			logger.Info("server starting (TLS)", slog.String("addr", srv.Addr))
			if err := srv.ListenAndServeTLS(cfg.TLSCertPath, cfg.TLSKeyPath); err != nil && err != http.ErrServerClosed {
				serverErr <- fmt.Errorf("listen and serve TLS: %w", err)
			}
		}()
	} else {
		go func() {
			logger.Info("server starting", slog.String("addr", srv.Addr))
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				serverErr <- fmt.Errorf("listen and serve: %w", err)
			}
		}()
	}

	select {
	case err := <-serverErr:
		return err
	case sig := <-quit:
		logger.Info("shutdown signal received", slog.String("signal", sig.String()))
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	logger.Info("shutting down gracefully", slog.Duration("timeout", shutdownTimeout))
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	logger.Info("server stopped")
	return nil
}

// --- Adapters bridging services to interfaces ---

// alertNotifierAdapter adapts NotificationService to auditsvc.AlertNotifier.
type alertNotifierAdapter struct {
	notifSvc *notifsvc.NotificationService
}

func (a *alertNotifierAdapter) Create(_ context.Context, params auditsvc.AlertNotification) {
	a.notifSvc.Create(context.Background(), notifsvc.CreateNotificationParams{
		Category: "alert",
		Severity: params.Severity,
		Title:    "Alert: " + params.RuleName,
		Body:     params.Description + " — triggered by " + params.Action + " from " + params.ActorLabel,
		Recipients: notifsvc.RecipientSpec{
			Role: "admin",
		},
	})
}

// smtpValidatorAdapter wraps the SMTP profile service to implement campaignsvc.SMTPValidator.
type smtpValidatorAdapter struct {
	svc *smtpsvc.Service
}

func (a *smtpValidatorAdapter) ValidateCampaignProfiles(ctx context.Context, campaignID string) ([]campaignsvc.SMTPValidationResult, error) {
	results, err := a.svc.ValidateCampaignProfiles(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	out := make([]campaignsvc.SMTPValidationResult, len(results))
	for i, r := range results {
		out[i] = campaignsvc.SMTPValidationResult{
			Success:     r.Success,
			ErrorDetail: r.ErrorDetail,
		}
	}
	return out, nil
}

// emailAuthValidatorAdapter wraps the DNS service to implement campaignsvc.EmailAuthValidator.
type emailAuthValidatorAdapter struct {
	svc *dnssvc.Service
}

func (a *emailAuthValidatorAdapter) ValidateEmailAuth(ctx context.Context, domainProfileID, actorID, actorLabel, sourceIP, correlationID string) (campaignsvc.EmailAuthResult, error) {
	result, err := a.svc.ValidateEmailAuth(ctx, domainProfileID, actorID, actorLabel, sourceIP, correlationID)
	if err != nil {
		return campaignsvc.EmailAuthResult{}, err
	}
	return campaignsvc.EmailAuthResult{
		SPFStatus:   result.SPFStatus,
		DKIMStatus:  result.DKIMStatus,
		DMARCStatus: result.DMARCStatus,
	}, nil
}

// blocklistCheckerAdapter adapts blocklistsvc.Service to targetsvc.BlocklistChecker.
type blocklistCheckerAdapter struct {
	svc *blocklistsvc.Service
}

func (a *blocklistCheckerAdapter) CheckEmail(ctx context.Context, email string) (targetsvc.BlocklistCheckResult, error) {
	result, err := a.svc.CheckEmail(ctx, email)
	if err != nil {
		return targetsvc.BlocklistCheckResult{}, err
	}
	pattern := ""
	if len(result.Matches) > 0 {
		pattern = result.Matches[0].Pattern
	}
	return targetsvc.BlocklistCheckResult{
		Blocked: result.Blocked,
		Pattern: pattern,
	}, nil
}

func buildRouter(cfg *config.Config, db *sql.DB, masterKey []byte, logger *slog.Logger) (http.Handler, *auditsvc.AuditService) { //nolint:funlen
	r := chi.NewRouter()

	// Separate rate-limit stores per traffic class (REQ-API-015, REQ-API-016).
	authRLStore := middleware.NewRateLimitStore()  // 10 req/min per IP
	readRLStore := middleware.NewRateLimitStore()  // 120 req/min per user
	writeRLStore := middleware.NewRateLimitStore() // 60 req/min per user

	authRL := middleware.RateLimit(authRLStore, 10, time.Minute, middleware.IPKey)
	readRL := middleware.RateLimit(readRLStore, 120, time.Minute, middleware.UserKey)
	writeRL := middleware.RateLimit(writeRLStore, 60, time.Minute, middleware.UserKey)

	// Global middleware (order matters).
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.CorrelationID)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.CORS(middleware.CORSConfig{
		AllowedOrigins: cfg.CORSAllowedOrigins,
	}))

	// Derive JWT signing key from master key.
	// DeriveSubkey only fails on empty master key, validated at startup.
	jwtSigningKey, err := crypto.DeriveSubkey(masterKey, crypto.PurposeJWTSigning)
	if err != nil {
		panic(fmt.Sprintf("derive JWT signing key: %v", err))
	}

	// Derive audit HMAC key from master key.
	auditHMACKey, err := crypto.DeriveSubkey(masterKey, crypto.PurposeHMACAudit)
	if err != nil {
		panic(fmt.Sprintf("derive audit HMAC key: %v", err))
	}

	auditHMACSvc := auditsvc.NewHMACService(auditHMACKey)
	auditService := auditsvc.NewAuditService(db, auditHMACSvc, 10_000)

	// JWT service: use RS256 if TACKLE_JWT_RSA_KEY_PATH is set, otherwise HS256.
	var jwtSvc *authsvc.JWTService
	if rsaKeyPath := os.Getenv("TACKLE_JWT_RSA_KEY_PATH"); rsaKeyPath != "" {
		rsaKeyData, err := os.ReadFile(rsaKeyPath)
		if err != nil {
			panic(fmt.Sprintf("read RSA key file: %v", err))
		}
		rsaKey, err := jwt.ParseRSAPrivateKeyFromPEM(rsaKeyData)
		if err != nil {
			panic(fmt.Sprintf("parse RSA private key: %v", err))
		}
		jwtSvc = authsvc.NewJWTServiceRS256(rsaKey, 15)
		slog.Info("jwt: using RS256 signing method")
	} else {
		jwtSvc = authsvc.NewJWTService(jwtSigningKey, 15)
	}
	blacklist := authsvc.NewTokenBlacklist()
	refreshSvc := authsvc.NewRefreshTokenService(db)
	loginRateLimiter := authsvc.NewRateLimiter()
	loginRateLimiter.SetLockoutCallback(func(key string, attempts int, lockedUntil time.Time) {
		_ = auditService.Log(context.Background(), auditsvc.LogEntry{
			Category:  auditsvc.CategoryUserActivity,
			Severity:  auditsvc.SeverityWarning,
			ActorType: auditsvc.ActorTypeSystem,
			Action:    "auth.rate_limit.triggered",
			Details:   map[string]any{"key": key, "attempts": attempts, "locked_until": lockedUntil.Format(time.RFC3339)},
		})
	})
	historySvc := authsvc.NewHistoryChecker(db)
	sessionConfigLoader := authsvc.NewSessionConfigLoader(db, 60*time.Second)

	authDeps := &authhandlers.Deps{
		DB:                  db,
		JWTSvc:              jwtSvc,
		RefreshSvc:          refreshSvc,
		Blacklist:           blacklist,
		HistoryChecker:      historySvc,
		RateLimiter:         loginRateLimiter,
		Policy:              authsvc.DefaultPolicy(),
		AuditSvc:            auditService,
		SessionConfigLoader: sessionConfigLoader,
	}

	roleDeps := &rolehandlers.Deps{DB: db, AuditSvc: auditService}
	userDeps := &userhandlers.Deps{DB: db, AuditSvc: auditService, RefreshSvc: refreshSvc}

	setupDeps := &setuphandlers.Deps{
		DB:         db,
		JWTSvc:     jwtSvc,
		RefreshSvc: refreshSvc,
		Policy:     authsvc.DefaultPolicy(),
		AuditSvc:   auditService,
	}

	auditDeps := &audithandlers.Deps{
		DB:       db,
		AuditSvc: auditService,
		HMACSvc:  auditHMACSvc,
	}

	settingsDeps := &settingshandlers.Deps{DB: db, AuditSvc: auditService}

	// Domain provider connections (Phase 2 - Domain Track).
	credEncSvc, err := credentials.NewEncryptionService(masterKey)
	if err != nil {
		panic(fmt.Sprintf("derive provider credentials key: %v", err))
	}
	dpRepo := repositories.NewDomainProviderRepository(db)
	dpService := dpsvc.NewService(dpRepo, credEncSvc, auditService)
	dpDeps := &dphandlers.Deps{Svc: dpService}

	hub := notifsvc.NewHub()
	go hub.Run()
	auditService.SetBroadcaster(hub)
	notifSvc := notifsvc.NewNotificationService(db, hub)
	notifEncSvc, err := crypto.NewEncryptionServiceForPurpose(masterKey, "tackle/notification-credentials")
	if err != nil {
		panic(fmt.Sprintf("notification encryption service: %v", err))
	}
	notifEmailSender := notifsvc.NewEmailSender(db, notifEncSvc)
	notifWebhookSender := notifsvc.NewWebhookSender(db, notifEncSvc)
	notifSvc.SetEmailSender(notifEmailSender)
	notifSvc.SetWebhookSender(notifWebhookSender)
	notifDeps := &notifhandlers.Deps{DB: db, NotifSvc: notifSvc, Hub: hub, JWTSvc: jwtSvc}

	// Alert evaluator for audit rule-based alerting.
	alertNotifier := &alertNotifierAdapter{notifSvc: notifSvc}
	alertEvaluator := auditsvc.NewAlertEvaluator(db, alertNotifier)
	auditService.SetAlertEvaluator(alertEvaluator)

	// Domain management (Phase 2 - Domain Track).
	domainProfileRepo := repositories.NewDomainProfileRepository(db)
	domainService := domainsvc.NewService(domainProfileRepo, dpRepo, credEncSvc, auditService, notifSvc, db)
	domainDeps := &domainhandlers.Deps{Svc: domainService}

	// DNS record management (Phase 2 - Domain Track).
	dnsRecordRepo := repositories.NewDNSRecordRepository(db)
	dnsService := dnssvc.NewService(dnsRecordRepo, domainProfileRepo, dpRepo, credEncSvc, auditService)
	dnsDeps := &dnshandlers.Deps{Svc: dnsService}

	// Domain health checks, categorization, and typosquat (Phase 2 - Domain Track).
	domainHealthRepo := repositories.NewDomainHealthRepository(db)
	healthService := healthsvc.NewService(domainHealthRepo, domainProfileRepo, dnsRecordRepo, auditService, notifSvc)
	catService := catsvc.NewService(domainHealthRepo, domainProfileRepo, auditService, notifSvc)
	typoService := typosvc.NewService(dpRepo, domainProfileRepo, credEncSvc, domainService, auditService)
	healthDeps := &healthhandlers.Deps{HealthSvc: healthService, CatSvc: catService, TypoSvc: typoService}

	// SMTP profiles (Phase 2 - Infrastructure Track).
	smtpEncSvc, err := credentials.NewSMTPEncryptionService(masterKey)
	if err != nil {
		panic(fmt.Sprintf("derive smtp credentials key: %v", err))
	}
	smtpProfileRepo := repositories.NewSMTPProfileRepository(db)
	smtpProfileService := smtpsvc.NewService(smtpProfileRepo, smtpEncSvc, auditService)
	smtpDeps := &smtphandlers.Deps{Svc: smtpProfileService}

	// External auth providers (Phase 2 - Infrastructure Track).
	authProvEncSvc, err := credentials.NewAuthProviderEncryptionService(masterKey)
	if err != nil {
		panic(fmt.Sprintf("derive auth provider credentials key: %v", err))
	}
	authProvRepo := repositories.NewAuthProviderRepository(db)
	roleMappingRepo := repositories.NewRoleMappingRepository(db)
	authIdentityRepo := repositories.NewAuthIdentityRepository(db)
	authProvService := authprovsvc.NewService(authProvRepo, roleMappingRepo, authIdentityRepo, authProvEncSvc, auditService)
	authProvProvSvc := authprovsvc.NewProvisioningService(db, authIdentityRepo, roleMappingRepo, auditService)
	authProvLinkSvc := authprovsvc.NewLinkingService(db, authIdentityRepo, auditService)
	authProvService.SetProvisioningService(authProvProvSvc)
	authProvService.SetLinkingService(authProvLinkSvc)

	// LoginRouter routes login attempts between local and LDAP auth.
	ldapProvider := authprovsvc.NewLDAPProvider()
	loginRouter := authprovsvc.NewLoginRouter(ldapProvider, authProvRepo, authProvProvSvc, authProvEncSvc)
	authDeps.LoginRouter = loginRouter

	authProvDeps := &authprovhandlers.Deps{
		Svc:        authProvService,
		ProvSvc:    authProvProvSvc,
		LinkSvc:    authProvLinkSvc,
		JWTSvc:     jwtSvc,
		RefreshSvc: refreshSvc,
	}

	// Cloud credentials and instance templates (Phase 2 - Infrastructure Track).
	cloudCredRepo := repositories.NewCloudCredentialRepository(db)
	instanceTmplRepo := repositories.NewInstanceTemplateRepository(db)
	cloudCredService := cloudsvc.NewService(cloudCredRepo, credEncSvc, auditService)
	instanceTmplService := tmplsvc.NewService(instanceTmplRepo, cloudCredRepo, credEncSvc, auditService)
	cloudDeps := &cloudhandlers.Deps{CredSvc: cloudCredService, TmplSvc: instanceTmplService}

	// Email templates (Phase 2 - Infrastructure Track).
	emailTmplRepo := repositories.NewEmailTemplateRepository(db)
	emailTmplService := emailtmplsvc.NewService(emailTmplRepo, auditService)
	attachmentSvc := emailtmplsvc.NewAttachmentService(db, "./data")
	etDeps := &ethandlers.Deps{Svc: emailTmplService, AttachmentSvc: attachmentSvc}

	// Targets (Phase 3 - Campaign Engine).
	targetRepo := repositories.NewTargetRepository(db)
	targetImportRepo := repositories.NewTargetImportRepository(db)
	mappingTmplRepo := repositories.NewMappingTemplateRepository(db)
	cteRepo := repositories.NewCampaignTargetEventRepository(db)
	targetService := targetsvc.NewService(targetRepo, targetImportRepo, mappingTmplRepo, cteRepo, auditService)
	targetDeps := &targethandlers.Deps{Svc: targetService}

	// Target groups, block list, canary targets (Phase 3 - Campaign Engine).
	targetGroupRepo := repositories.NewTargetGroupRepository(db)
	blocklistRepo := repositories.NewBlocklistRepository(db)
	overrideRepo := repositories.NewBlocklistOverrideRepository(db)
	canaryRepo := repositories.NewCanaryTargetRepository(db)
	groupService := targetgroupsvc.NewService(targetGroupRepo, auditService)
	blocklistService := blocklistsvc.NewService(blocklistRepo, overrideRepo, auditService, notifSvc)
	tgDeps := &tghandlers.Deps{GroupSvc: groupService, BlocklistSvc: blocklistService, CanaryRepo: canaryRepo}

	// Wire blocklist checker into target service for import validation.
	targetService.SetBlocklistChecker(&blocklistCheckerAdapter{svc: blocklistService})

	// Landing pages (Phase 3 - Campaign Engine).
	lpRepo := repositories.NewLandingPageRepository(db)
	lpService := lpsvc.NewService(lpRepo, auditService)

	// Compilation engine and app manager.
	buildBaseDir := os.TempDir() + "/tackle-builds"
	_ = os.MkdirAll(buildBaseDir, 0755)
	compilationEngine := compiler.NewCompilationEngine(lpRepo, auditService, compiler.EngineConfig{
		BuildBaseDir:     buildBaseDir,
		FrameworkBaseURL: "http://127.0.0.1:" + cfg.ListenAddr,
		Logger:           logger,
	})
	appManager := hosting.NewAppManager(lpRepo, hosting.ManagerConfig{Logger: logger})
	appManager.CleanupStaleBuilds(context.Background())

	lpDeps := &lphandlers.Deps{Svc: lpService, Engine: compilationEngine, AppMgr: appManager}

	// Internal API handlers (generated app -> framework communication).
	internalDeps := &internalhandlers.Deps{
		DB:        db,
		EventRepo: cteRepo,
		BuildRepo: lpRepo,
		AuditSvc:  auditService,
	}
	// Wire HandleSessionCapture method (will be auto-wired via reflection).
	var _ = internalDeps.HandleSessionCapture

	// Campaigns (Phase 3 - Campaign Engine).
	campaignRepo := repositories.NewCampaignRepository(db)
	campaignService := campaignsvc.NewService(campaignRepo, auditService)
	approvalRepo := repositories.NewCampaignApprovalRepository(db)
	approvalService := campaignsvc.NewApprovalService(campaignRepo, approvalRepo, auditService, notifSvc, blocklistService, groupService)
	campaignDeps := &campaignhandlers.Deps{DB: db, Svc: campaignService, ApprovalSvc: approvalService}

	// Endpoint management (Phase 3 - Campaign Engine).
	epEncSvc, err := crypto.NewEncryptionService(masterKey)
	if err != nil {
		panic(fmt.Sprintf("derive endpoint encryption key: %v", err))
	}
	endpointRepo := repositories.NewPhishingEndpointRepository(db)
	endpointSM := endpoint.NewStateMachine(endpointRepo, auditService)
	endpointIPPool := cloud.NewIPPoolAllocator(endpointRepo)
	endpointProvisioner := endpoint.NewProvisioner(endpointSM, endpointRepo, endpointIPPool, auditService)
	endpointCommAuth := endpoint.NewCommAuthService(endpointRepo, epEncSvc, auditService)
	endpointSSHDeployer := endpoint.NewSSHDeployer(endpointRepo, endpointSM, epEncSvc, auditService, &endpoint.RealSSHClientFactory{})
	endpointProvisioner.SetDeployer(endpointSSHDeployer, endpointCommAuth)
	// Wire IP change callback after endpointMgmtService is created (below).
	endpointMgmtService := endpointmgmtsvc.NewService(
		db, endpointRepo, endpointSM, endpointProvisioner, endpointCommAuth,
		epEncSvc, auditService, notifSvc,
	)
	endpointMgmtService.SetHub(hub)
	endpointProvisioner.SetIPChangeCallback(endpointMgmtService.CheckIPChange)
	endpointProviderResolver := endpointhandlers.NewDBProviderResolver(endpointRepo, cloudCredRepo, credEncSvc)
	endpointDNSUpdater := endpointhandlers.NewLazyDNSUpdater(dpRepo, credEncSvc)
	endpointDeps := &endpointhandlers.Deps{
		Svc:              endpointMgmtService,
		ProviderResolver: endpointProviderResolver,
		DNSUpdater:       endpointDNSUpdater,
	}
	provisionDeps := &endpointhandlers.ProvisionDeps{
		Provisioner:   endpointProvisioner,
		CommAuth:      endpointCommAuth,
		CampaignRepo:  campaignRepo,
		CloudCredRepo: cloudCredRepo,
		TemplateRepo:  instanceTmplRepo,
		CredEncSvc:    credEncSvc,
		AuditSvc:      auditService,
		DNSUpdater:    endpointDNSUpdater,
	}
	redeployDeps := &endpointhandlers.RedeployDeps{
		Svc:      endpointMgmtService,
		Deployer: endpointSSHDeployer,
		CommAuth: endpointCommAuth,
		AuditSvc: auditService,
	}

	// Credential capture (Phase 3 - Campaign Engine).
	captureEncSvc, err := crypto.NewEncryptionServiceForPurpose(masterKey, credsvc.PurposeCredentialEncryption)
	if err != nil {
		panic(fmt.Sprintf("derive credential encryption key: %v", err))
	}
	captureEventRepo := repositories.NewCaptureEventRepository(db)
	credService := credsvc.NewService(db, captureEventRepo, captureEncSvc, auditService, notifSvc, campaignRepo)
	credDeps := &credhandlers.Deps{Svc: credService}

	// Wire credential service into internal API handlers.
	internalDeps.CredSvc = credService

	// Email delivery service (Phase 3 - Campaign Engine).
	emailDeliveryService := emaildeliverysvc.NewService(
		campaignRepo, targetRepo, smtpProfileRepo, emailTmplRepo,
		auditService, notifSvc, emaildeliverysvc.DefaultConfig(), logger,
	)
	emailDeliveryDeps := &emaildeliveryhandlers.Deps{Svc: emailDeliveryService}

	// Wire email delivery into internal API for endpoint callbacks.
	internalDeps.EmailDeliverySvc = emailDeliveryService
	etDeps.EmailDeliverySvc = emailDeliveryService // ECOMP-09: Wire send-test functionality.

	// Wire email delivery into campaign service for post-transition hooks.
	campaignService.SetEmailDeliveryHook(emailDeliveryService)

	// Wire additional validation dependencies into approval service.
	approvalService.SetValidationDeps(smtpProfileRepo, domainProfileRepo, lpRepo, campaignService)

	// Derive tracking token secret for deterministic token generation.
	trackingSecret, err := crypto.DeriveSubkey(masterKey, crypto.PurposeTrackingToken)
	if err != nil {
		panic(fmt.Sprintf("derive tracking token secret: %v", err))
	}

	// Wire tracking token service into internal API handlers for token resolution.
	trackingHMAC := crypto.NewHMACService(trackingSecret)
	tokenSvc := tracking.NewTokenService(trackingHMAC)
	internalDeps.TokenSvc = tokenSvc

	// Campaign build orchestrator — connects landing page compilation,
	// endpoint provisioning, DNS, TLS into an automated workflow.
	campaignBuilder := campaignsvc.NewCampaignBuilder(campaignsvc.BuilderDeps{
		CampaignRepo:       campaignRepo,
		TargetGroupRepo:    targetGroupRepo,
		GroupSvc:           groupService,
		BlocklistSvc:       blocklistService,
		CampaignSvc:        campaignService,
		Engine:             compilationEngine,
		AppMgr:             appManager,
		Provisioner:        endpointProvisioner,
		DNSUpdater:         endpointDNSUpdater,
		TLSSvc:             nil, // ACMEService wired in a future session
		AuditSvc:           auditService,
		NotifSvc:           notifSvc,
		Hub:                hub,
		TrackingSecret:     trackingSecret,
		SMTPValidator:      &smtpValidatorAdapter{svc: smtpProfileService},
		EmailAuthValidator: &emailAuthValidatorAdapter{svc: dnsService},
	})
	campaignDeps.Builder = campaignBuilder

	// Metrics aggregation service.
	metricsService := metricssvc.NewService(db)
	metricsDeps := &metricshandlers.Deps{Svc: metricsService}

	// AI integration (all endpoints stub — deferred).
	aiDeps := &aihandlers.Deps{}

	// Report generation service.
	reportOutputDir := os.Getenv("TACKLE_REPORT_DIR")
	if reportOutputDir == "" {
		reportOutputDir = "data/reports"
	}
	reportTemplateSvc := reportsvc.NewTemplateService(db)
	reportGenerator := reportsvc.NewGenerator(db, metricsService, reportOutputDir, logger)
	reportDeps := &reporthandlers.Deps{TemplateSvc: reportTemplateSvc, Generator: reportGenerator}

	searchDeps := &searchhandlers.Deps{DB: db}

	// Campaign teardown service — handles infrastructure cleanup on completion/unlock.
	teardownSvc := campaignsvc.NewTeardownService(campaignsvc.TeardownDeps{
		CampaignRepo:    campaignRepo,
		EndpointRepo:    endpointRepo,
		LandingPageRepo: lpRepo,
		AppMgr:          appManager,
		AuditSvc:        auditService,
	})
	campaignService.SetTeardownService(teardownSvc)

	// Start background workers.
	workerCtx, _ := context.WithCancel(context.Background()) //nolint:govet
	go workers.NewHealthCheckWorker(healthService, 6*time.Hour, logger).Start(workerCtx)
	go workers.NewCategorizationWorker(catService, 24*time.Hour, logger).Start(workerCtx)
	go workers.NewCampaignAutoLaunchWorker(campaignService, 30*time.Second, logger).Start(workerCtx)
	go workers.NewEndpointHealthWorker(endpointMgmtService, db, 30*time.Second, logger).Start(workerCtx)
	go workers.NewCampaignCompletionWorker(campaignService, campaignRepo, auditService, 60*time.Second, logger).Start(workerCtx)
	go workers.NewDomainExpiryWorker(domainService, 24*time.Hour, logger).Start(workerCtx)
	go workers.NewTargetPurgeWorker(targetService, 365, 24*time.Hour, logger).Start(workerCtx)
	appManager.StartHealthMonitor(workerCtx)

	// API key authentication service.
	apiKeySvc := apikeysvc.NewService(db)
	apiKeyDeps := &apikeyhandlers.Deps{Svc: apiKeySvc, AuditSvc: auditService}
	apiKeyAuth := middleware.APIKeyAuth(apiKeySvc)

	endpointTokenCache := middleware.NewEndpointTokenCache(endpointRepo, epEncSvc)
	requireEndpointAuth := middleware.RequireEndpointAuth(endpointTokenCache)

	userStatusCache := middleware.NewUserStatusCache(db)
	requireAuth := middleware.RequireAuth(jwtSvc, blacklist, userStatusCache)
	requirePerm := middleware.RequirePermission
	requireSetupComplete := middleware.RequireSetupComplete(db)

	r.Route("/api/v1", func(r chi.Router) {
		// Public endpoints — no auth, no setup check.
		r.Get("/health", handlers.Health(buildVersion))
		r.Get("/auth/providers", authProvDeps.GetEnabledProviders)

		// OIDC/FusionAuth login flow (public, rate-limited with authRL below).
		r.With(authRL).Get("/auth/oidc/{providerID}/login", authProvDeps.OIDCLogin)
		r.With(authRL).Get("/auth/oidc/callback/{providerID}", authProvDeps.OIDCCallback)

		// Setup endpoints — public but guarded by setup state.
		r.With(middleware.RequireSetupPending(db)).Post("/setup", setupDeps.Setup)
		r.Get("/setup/status", setupDeps.Status)

		// Auth endpoints: rate limited per IP, body size limited.
		r.Group(func(r chi.Router) {
			r.Use(authRL)
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					req.Body = http.MaxBytesReader(w, req.Body, bodySizeStandard)
					next.ServeHTTP(w, req)
				})
			})
			r.With(requireSetupComplete).Post("/auth/login", authDeps.Login)
			r.With(requireSetupComplete).Post("/auth/refresh", authDeps.Refresh)
		})

		// Authenticated endpoints.
		r.Group(func(r chi.Router) {
			r.Use(requireSetupComplete)
			r.Use(apiKeyAuth)
			r.Use(requireAuth)
			r.Use(middleware.CSRFProtection)

			// Auth management (no extra permission, any authenticated user).
			r.Post("/auth/logout", authDeps.Logout)
			r.Get("/auth/me", authDeps.Me)

			// Own session / profile management.
			r.With(readRL).Get("/users/me/sessions", authDeps.ListSessions)
			r.With(writeRL, bodySizeMW(bodySizeStandard)).Put("/users/me/password", authDeps.ChangePassword)
			r.With(writeRL).Delete("/users/me/sessions/{id}", authDeps.DeleteSession)
			r.With(writeRL, bodySizeMW(bodySizeStandard)).Put("/users/me/profile", userDeps.UpdateOwnProfile)
			r.With(readRL).Get("/users/me/preferences", userDeps.GetPreferences)
			r.With(writeRL, bodySizeMW(bodySizeStandard)).Put("/users/me/preferences", userDeps.UpdatePreferences)

			// Global cross-entity search.
			r.With(readRL).Get("/search", searchDeps.Search)

			// Permissions endpoint.
			r.With(readRL, requirePerm("roles:read")).Get("/permissions", permhandlers.List)

			// Role management endpoints.
			r.With(readRL, requirePerm("roles:read")).Get("/roles", roleDeps.List)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("roles:create")).Post("/roles", roleDeps.Create)
			r.With(readRL, requirePerm("roles:read")).Get("/roles/{id}", roleDeps.Get)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("roles:update")).Put("/roles/{id}", roleDeps.Update)
			r.With(writeRL, requirePerm("roles:delete")).Delete("/roles/{id}", roleDeps.Delete)
			r.With(readRL, requirePerm("roles:read")).Get("/roles/{id}/users", roleDeps.Users)

			// User CRUD.
			r.With(readRL, requirePerm("users:read")).Get("/users", userDeps.ListUsers)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("users:create")).Post("/users", userDeps.CreateUser)
			r.With(readRL, requirePerm("users:read")).Get("/users/{id}", userDeps.GetUser)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("users:update")).Put("/users/{id}", userDeps.UpdateUser)
			r.With(writeRL, requirePerm("users:delete")).Delete("/users/{id}", userDeps.DeleteUser)
			r.With(readRL, requirePerm("users:read")).Get("/users/{id}/activity", userDeps.GetActivity)
			r.With(readRL, requirePerm("users:read")).Get("/users/{id}/sessions", userDeps.ListUserSessions)
			r.With(writeRL, requirePerm("users:update")).Delete("/users/{id}/sessions/{sid}", userDeps.TerminateUserSession)

			// Admin password reset — requires users:update.
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("users:update")).Put("/users/{id}/password", authDeps.ResetPassword)

			// User role assignment.
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("users:update")).Put("/users/{id}/roles", userDeps.AssignRole)

			// Audit log endpoints.
			r.With(readRL, requirePerm("logs.audit:read")).Get("/logs/audit", auditDeps.List)
			r.With(readRL, requirePerm("logs.audit:read")).Get("/logs/audit/{id}", auditDeps.Get)
			r.With(writeRL, requirePerm("logs.audit:read")).Post("/logs/audit/{id}/verify", auditDeps.Verify)

			// Alert rule endpoints (admin only — guarded by settings:update permission).
			r.With(readRL, requirePerm("settings:read")).Get("/alert-rules", auditDeps.ListAlertRules)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("settings:update")).Post("/alert-rules", auditDeps.CreateAlertRule)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("settings:update")).Put("/alert-rules/{id}", auditDeps.UpdateAlertRule)
			r.With(writeRL, requirePerm("settings:update")).Delete("/alert-rules/{id}", auditDeps.DeleteAlertRule)

			// Settings endpoints.
			r.With(readRL, requirePerm("settings:read")).Get("/settings", settingsDeps.Get)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("settings:update")).Put("/settings", settingsDeps.Update)

			// External auth provider configuration endpoints (Phase 2 - Infrastructure Track).
			r.With(readRL, requirePerm("settings:read")).Get("/settings/auth-providers", authProvDeps.ListProviders)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("settings:update")).Post("/settings/auth-providers", authProvDeps.CreateProvider)
			r.With(readRL, requirePerm("settings:read")).Get("/settings/auth-providers/{id}", authProvDeps.GetProvider)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("settings:update")).Put("/settings/auth-providers/{id}", authProvDeps.UpdateProvider)
			r.With(writeRL, requirePerm("settings:update")).Delete("/settings/auth-providers/{id}", authProvDeps.DeleteProvider)
			r.With(writeRL, requirePerm("settings:update")).Post("/settings/auth-providers/{id}/test", authProvDeps.TestProvider)
			r.With(writeRL, requirePerm("settings:update")).Post("/settings/auth-providers/{id}/enable", authProvDeps.EnableProvider)
			r.With(writeRL, requirePerm("settings:update")).Post("/settings/auth-providers/{id}/disable", authProvDeps.DisableProvider)
			r.With(readRL, requirePerm("settings:read")).Get("/settings/auth-providers/{id}/role-mappings", authProvDeps.GetRoleMappings)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("settings:update")).Put("/settings/auth-providers/{id}/role-mappings", authProvDeps.SetRoleMappings)

			// API key management endpoints.
			r.With(readRL, requirePerm("api_keys:read")).Get("/api-keys", apiKeyDeps.List)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("api_keys:create")).Post("/api-keys", apiKeyDeps.Create)
			r.With(writeRL, requirePerm("api_keys:delete")).Delete("/api-keys/{id}", apiKeyDeps.Revoke)

			// Account linking endpoints (any authenticated user).
			r.With(writeRL).Post("/auth/link/{providerID}", authProvDeps.InitiateLink)
			r.With(authRL).Get("/auth/link/callback/{providerID}", authProvDeps.LinkCallback)
			r.With(writeRL).Delete("/auth/identities/{identityID}", authProvDeps.UnlinkIdentity)
			r.With(readRL).Get("/auth/identities", authProvDeps.ListIdentities)

			// Domain provider connection endpoints (Phase 2).
			r.With(readRL, requirePerm("domains:read")).Get("/settings/domain-providers", dpDeps.List)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("domains:create")).Post("/settings/domain-providers", dpDeps.Create)
			r.With(readRL, requirePerm("domains:read")).Get("/settings/domain-providers/{id}", dpDeps.Get)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("domains:update")).Put("/settings/domain-providers/{id}", dpDeps.Update)
			r.With(writeRL, requirePerm("domains:delete")).Delete("/settings/domain-providers/{id}", dpDeps.Delete)
			r.With(writeRL, requirePerm("domains:update")).Post("/settings/domain-providers/{id}/test", dpDeps.TestConn)

			// Domain management endpoints (Phase 2).
			r.With(writeRL, requirePerm("domains:create")).Post("/domains/sync", domainDeps.SyncAll)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("domains:create")).Post("/domains", domainDeps.Create)
			r.With(readRL, requirePerm("domains:read")).Get("/domains", domainDeps.List)
			r.With(readRL, requirePerm("domains:read")).Get("/domains/{id}", domainDeps.Get)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("domains:update")).Put("/domains/{id}", domainDeps.Update)
			r.With(writeRL, requirePerm("domains:delete")).Delete("/domains/{id}", domainDeps.Delete)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("domains:read")).Post("/domains/check-availability", domainDeps.CheckAvailability)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("domains:update")).Post("/domains/{id}/renew", domainDeps.Renew)
			r.With(readRL, requirePerm("domains:read")).Get("/domains/{id}/renewal-history", domainDeps.RenewalHistory)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("domains:create")).Post("/domains/registration-requests", domainDeps.SubmitRegistrationRequest)
			r.With(writeRL, requirePerm("domains:create")).Post("/domains/registration-requests/{id}/approve", domainDeps.ApproveRegistration)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("domains:create")).Post("/domains/registration-requests/{id}/reject", domainDeps.RejectRegistration)

			// DNS record management endpoints (Phase 2).
			r.With(readRL, requirePerm("domains:read")).Get("/domains/{id}/dns-records", dnsDeps.ListRecords)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("domains:create")).Post("/domains/{id}/dns-records", dnsDeps.CreateRecord)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("domains:update")).Put("/domains/{id}/dns-records/{recordId}", dnsDeps.UpdateRecord)
			r.With(writeRL, requirePerm("domains:delete")).Delete("/domains/{id}/dns-records/{recordId}", dnsDeps.DeleteRecord)
			r.With(readRL, requirePerm("domains:read")).Get("/domains/{id}/dns-records/soa", dnsDeps.GetSOA)
			r.With(readRL, requirePerm("domains:read")).Get("/domains/{id}/email-auth", dnsDeps.GetEmailAuth)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("domains:update")).Post("/domains/{id}/email-auth/spf", dnsDeps.ConfigureSPF)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("domains:update")).Post("/domains/{id}/email-auth/dkim", dnsDeps.GenerateDKIM)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("domains:update")).Post("/domains/{id}/email-auth/dmarc", dnsDeps.ConfigureDMARC)
			r.With(writeRL, requirePerm("domains:update")).Post("/domains/{id}/email-auth/validate", dnsDeps.ValidateEmailAuth)
			r.With(readRL, requirePerm("domains:read")).Get("/domains/{id}/propagation-checks", dnsDeps.GetPropagationChecks)

			// Domain health check endpoints (Phase 2).
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("domains:read")).Post("/domains/{id}/health-check", healthDeps.TriggerHealthCheck)
			r.With(readRL, requirePerm("domains:read")).Get("/domains/{id}/health-checks", healthDeps.ListHealthChecks)
			r.With(readRL, requirePerm("domains:read")).Get("/domains/{id}/health-checks/latest", healthDeps.GetLatestHealthCheck)
			r.With(readRL, requirePerm("domains:read")).Get("/domains/{id}/categorization", healthDeps.GetLatestCategorization)
			r.With(readRL, requirePerm("domains:read")).Get("/domains/{id}/categorization/history", healthDeps.GetCategorizationHistory)
			r.With(writeRL, requirePerm("domains:read")).Post("/domains/{id}/categorization/check", healthDeps.TriggerCategorization)

			// Typosquat tool endpoints (Phase 2).
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("domains:read")).Post("/tools/typosquat", healthDeps.GenerateTyposquats)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("domains:create")).Post("/tools/typosquat/register", healthDeps.RegisterTyposquat)

			// Cloud credential endpoints (Phase 2 - Infrastructure Track).
			r.With(readRL, requirePerm("infrastructure:read")).Get("/settings/cloud-credentials", cloudDeps.ListCredentials)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("infrastructure:create")).Post("/settings/cloud-credentials", cloudDeps.CreateCredential)
			r.With(readRL, requirePerm("infrastructure:read")).Get("/settings/cloud-credentials/{id}", cloudDeps.GetCredential)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("infrastructure:update")).Put("/settings/cloud-credentials/{id}", cloudDeps.UpdateCredential)
			r.With(writeRL, requirePerm("infrastructure:delete")).Delete("/settings/cloud-credentials/{id}", cloudDeps.DeleteCredential)
			r.With(writeRL, requirePerm("infrastructure:update")).Post("/settings/cloud-credentials/{id}/test", cloudDeps.TestCredential)

			// Instance template endpoints (Phase 2 - Infrastructure Track).
			r.With(readRL, requirePerm("infrastructure:read")).Get("/instance-templates", cloudDeps.ListTemplates)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("infrastructure:create")).Post("/instance-templates", cloudDeps.CreateTemplate)
			r.With(readRL, requirePerm("infrastructure:read")).Get("/instance-templates/{id}", cloudDeps.GetTemplate)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("infrastructure:update")).Put("/instance-templates/{id}", cloudDeps.UpdateTemplate)
			r.With(writeRL, requirePerm("infrastructure:delete")).Delete("/instance-templates/{id}", cloudDeps.DeleteTemplate)
			r.With(readRL, requirePerm("infrastructure:read")).Get("/instance-templates/{id}/versions", cloudDeps.ListTemplateVersions)
			r.With(readRL, requirePerm("infrastructure:read")).Get("/instance-templates/{id}/versions/{version}", cloudDeps.GetTemplateVersion)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("infrastructure:read")).Post("/instance-templates/validate", cloudDeps.ValidateTemplate)

			// SMTP profile endpoints (Phase 2 - Infrastructure Track).
			r.With(readRL, requirePerm("infrastructure:read")).Get("/smtp-profiles", smtpDeps.ListProfiles)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("infrastructure:create")).Post("/smtp-profiles", smtpDeps.CreateProfile)
			r.With(readRL, requirePerm("infrastructure:read")).Get("/smtp-profiles/{id}", smtpDeps.GetProfile)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("infrastructure:update")).Put("/smtp-profiles/{id}", smtpDeps.UpdateProfile)
			r.With(writeRL, requirePerm("infrastructure:delete")).Delete("/smtp-profiles/{id}", smtpDeps.DeleteProfile)
			r.With(writeRL, requirePerm("infrastructure:update")).Post("/smtp-profiles/{id}/test", smtpDeps.TestProfile)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("infrastructure:create")).Post("/smtp-profiles/{id}/duplicate", smtpDeps.DuplicateProfile)

			// Campaign SMTP configuration endpoints (Phase 2 - ready for Phase 3 campaign integration).
			r.With(readRL, requirePerm("infrastructure:read")).Get("/campaigns/{id}/smtp-profiles", smtpDeps.ListCampaignProfiles)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("infrastructure:update")).Post("/campaigns/{id}/smtp-profiles", smtpDeps.AssignProfile)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("infrastructure:update")).Put("/campaigns/{id}/smtp-profiles/{assocId}", smtpDeps.UpdateCampaignAssociation)
			r.With(writeRL, requirePerm("infrastructure:update")).Delete("/campaigns/{id}/smtp-profiles/{assocId}", smtpDeps.RemoveCampaignAssociation)
			r.With(readRL, requirePerm("infrastructure:read")).Get("/campaigns/{id}/send-schedule", smtpDeps.GetSendSchedule)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("infrastructure:update")).Put("/campaigns/{id}/send-schedule", smtpDeps.UpsertSendSchedule)
			r.With(writeRL, requirePerm("infrastructure:read")).Post("/campaigns/{id}/smtp-profiles/validate", smtpDeps.ValidateCampaignProfiles)

			// Email template endpoints (Phase 2 - Infrastructure Track).
			r.With(readRL, requirePerm("templates.email:read")).Get("/email-templates", etDeps.ListTemplates)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("templates.email:create")).Post("/email-templates", etDeps.CreateTemplate)
			r.With(readRL, requirePerm("templates.email:read")).Get("/email-templates/{id}", etDeps.GetTemplate)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("templates.email:update")).Put("/email-templates/{id}", etDeps.UpdateTemplate)
			r.With(writeRL, requirePerm("templates.email:delete")).Delete("/email-templates/{id}", etDeps.DeleteTemplate)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("templates.email:create")).Post("/email-templates/{id}/clone", etDeps.CloneTemplate)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("templates.email:read")).Post("/email-templates/{id}/preview", etDeps.PreviewTemplate)
			r.With(writeRL, requirePerm("templates.email:read")).Post("/email-templates/{id}/validate", etDeps.ValidateTemplate)
			r.With(readRL, requirePerm("templates.email:read")).Get("/email-templates/{id}/versions", etDeps.ListVersions)
			r.With(readRL, requirePerm("templates.email:read")).Get("/email-templates/{id}/versions/{version}", etDeps.GetVersion)
			r.With(readRL, requirePerm("templates.email:export")).Get("/email-templates/{id}/export", etDeps.ExportTemplate)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("templates.email:create")).Post("/email-templates/{id}/send-test", etDeps.SendTestEmail)

			// Email template attachments.
			r.With(readRL, requirePerm("templates.email:read")).Get("/email-templates/{id}/attachments", etDeps.ListAttachments)
			r.With(writeRL, bodySizeMW(12<<20), requirePerm("templates.email:update")).Post("/email-templates/{id}/attachments", etDeps.UploadAttachment)
			r.With(writeRL, requirePerm("templates.email:update")).Delete("/email-templates/{id}/attachments/{aid}", etDeps.DeleteAttachment)

			// Target endpoints (Phase 3 - Campaign Engine).
			r.With(readRL, requirePerm("targets:read")).Get("/targets", targetDeps.ListTargets)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("targets:create")).Post("/targets", targetDeps.CreateTarget)
			r.With(readRL, requirePerm("targets:read")).Get("/targets/check-email", targetDeps.CheckEmail)
			r.With(readRL, requirePerm("targets:read")).Get("/targets/departments", targetDeps.GetDepartments)
			r.With(readRL, requirePerm("targets:read")).Get("/targets/{id}", targetDeps.GetTarget)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("targets:update")).Put("/targets/{id}", targetDeps.UpdateTarget)
			r.With(writeRL, requirePerm("targets:delete")).Delete("/targets/{id}", targetDeps.DeleteTarget)
			r.With(writeRL, requirePerm("targets:update")).Post("/targets/{id}/restore", targetDeps.RestoreTarget)
			r.With(readRL, requirePerm("targets:read")).Get("/targets/{id}/history", targetDeps.GetHistory)
			r.With(readRL, requirePerm("targets:read")).Get("/targets/{id}/events", targetDeps.GetEvents)
			r.With(readRL, requirePerm("targets:read")).Get("/targets/{id}/stats", targetDeps.GetStats)

			// Target CSV import endpoints.
			r.With(writeRL, requirePerm("targets:create")).Post("/targets/import/upload", targetDeps.UploadCSV)
			r.With(readRL, requirePerm("targets:read")).Get("/targets/import/{upload_id}/preview", targetDeps.GetImportPreview)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("targets:create")).Post("/targets/import/{upload_id}/mapping", targetDeps.SubmitMapping)
			r.With(writeRL, requirePerm("targets:create")).Post("/targets/import/{upload_id}/validate", targetDeps.ValidateImport)
			r.With(writeRL, requirePerm("targets:create")).Post("/targets/import/{upload_id}/commit", targetDeps.CommitImport)

			// Target import mapping templates.
			r.With(readRL, requirePerm("targets:read")).Get("/targets/import/mapping-templates", targetDeps.ListMappingTemplates)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("targets:create")).Post("/targets/import/mapping-templates", targetDeps.CreateMappingTemplate)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("targets:update")).Put("/targets/import/mapping-templates/{id}", targetDeps.UpdateMappingTemplate)
			r.With(writeRL, requirePerm("targets:delete")).Delete("/targets/import/mapping-templates/{id}", targetDeps.DeleteMappingTemplate)

			// Target bulk operations.
			r.With(writeRL, bodySizeMW(bodySizeBatch), requirePerm("targets:delete")).Post("/targets/bulk/delete", targetDeps.BulkDelete)
			r.With(writeRL, bodySizeMW(bodySizeBatch), requirePerm("targets:update")).Post("/targets/bulk/edit", targetDeps.BulkEdit)
			r.With(writeRL, bodySizeMW(bodySizeBatch), requirePerm("targets:export")).Post("/targets/bulk/export", targetDeps.BulkExport)
			r.With(writeRL, bodySizeMW(bodySizeBatch), requirePerm("targets:update")).Post("/targets/bulk/add-to-group", tgDeps.BulkAddToGroup)
			r.With(writeRL, bodySizeMW(bodySizeBatch), requirePerm("targets:update")).Post("/targets/bulk/remove-from-group", tgDeps.BulkRemoveFromGroup)

			// Campaign target timeline (nested under campaigns).
			r.With(readRL, requirePerm("targets:read")).Get("/campaigns/{id}/targets/{target_id}/timeline", targetDeps.GetTimeline)

			// Target group endpoints (Phase 3 - Campaign Engine).
			r.With(readRL, requirePerm("targets:read")).Get("/target-groups", tgDeps.ListGroups)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("targets:create")).Post("/target-groups", tgDeps.CreateGroup)
			r.With(readRL, requirePerm("targets:read")).Get("/target-groups/{id}", tgDeps.GetGroup)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("targets:update")).Put("/target-groups/{id}", tgDeps.UpdateGroup)
			r.With(writeRL, requirePerm("targets:delete")).Delete("/target-groups/{id}", tgDeps.DeleteGroup)
			r.With(writeRL, bodySizeMW(bodySizeBatch), requirePerm("targets:create")).Post("/target-groups/{id}/members", tgDeps.AddMembers)
			r.With(writeRL, bodySizeMW(bodySizeBatch), requirePerm("targets:update")).Delete("/target-groups/{id}/members", tgDeps.RemoveMembers)

			// Campaign group assignment and resolution endpoints.
			r.With(readRL, requirePerm("targets:read")).Get("/campaigns/{id}/target-groups", tgDeps.ListCampaignGroups)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("targets:create")).Post("/campaigns/{id}/target-groups", tgDeps.AssignGroup)
			r.With(writeRL, requirePerm("targets:update")).Delete("/campaigns/{id}/target-groups/{groupId}", tgDeps.UnassignGroup)
			r.With(readRL, requirePerm("targets:read")).Get("/campaigns/{id}/resolve-targets", tgDeps.ResolveTargets)

			// Block list endpoints (Phase 3 - Campaign Engine).
			r.With(readRL, requirePerm("targets:read")).Get("/blocklist", tgDeps.ListBlocklist)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("blocklist:manage")).Post("/blocklist", tgDeps.CreateBlocklistEntry)
			r.With(readRL, requirePerm("targets:read")).Get("/blocklist/{id}", tgDeps.GetBlocklistEntry)
			r.With(writeRL, requirePerm("blocklist:manage")).Put("/blocklist/{id}/deactivate", tgDeps.DeactivateBlocklistEntry)
			r.With(writeRL, requirePerm("blocklist:manage")).Put("/blocklist/{id}/reactivate", tgDeps.ReactivateBlocklistEntry)
			r.With(readRL, requirePerm("targets:read")).Get("/blocklist/check", tgDeps.CheckBlocklist)

			// Block list override management endpoints.
			r.With(readRL, requirePerm("blocklist:manage")).Get("/blocklist-overrides", tgDeps.ListOverrides)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("blocklist:manage")).Post("/blocklist-overrides/{id}/decide", tgDeps.DecideOverride)

			// Block list review and override endpoints.
			r.With(readRL, requirePerm("targets:read")).Get("/campaigns/{id}/blocklist-review", tgDeps.GetBlocklistReview)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("blocklist:manage")).Post("/campaigns/{id}/blocklist-override", tgDeps.BlocklistOverride)

			// Canary target endpoints.
			r.With(writeRL, bodySizeMW(bodySizeBatch), requirePerm("targets:create")).Post("/campaigns/{id}/canary-targets", tgDeps.DesignateCanary)
			r.With(writeRL, bodySizeMW(bodySizeBatch), requirePerm("targets:update")).Delete("/campaigns/{id}/canary-targets", tgDeps.UndesignateCanary)
			r.With(readRL, requirePerm("targets:read")).Get("/campaigns/{id}/canary-targets", tgDeps.ListCanary)

			// Landing page endpoints (Phase 3 - Campaign Engine).
			r.With(readRL, requirePerm("landing_pages:read")).Get("/landing-pages", lpDeps.ListProjects)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("landing_pages:create")).Post("/landing-pages", lpDeps.CreateProject)
			r.With(readRL, requirePerm("landing_pages:read")).Get("/landing-pages/templates", lpDeps.ListTemplates)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("landing_pages:create")).Post("/landing-pages/templates", lpDeps.SaveAsTemplate)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("landing_pages:update")).Put("/landing-pages/templates/{templateId}", lpDeps.UpdateTemplate)
			r.With(writeRL, requirePerm("landing_pages:delete")).Delete("/landing-pages/templates/{templateId}", lpDeps.DeleteTemplate)
			r.With(readRL, requirePerm("landing_pages:read")).Get("/landing-pages/components", lpDeps.ListComponents)
			r.With(readRL, requirePerm("landing_pages:read")).Get("/landing-pages/themes", lpDeps.ListThemes)
			r.With(readRL, requirePerm("landing_pages:read")).Get("/landing-pages/js-snippets", lpDeps.ListJSSnippets)
			r.With(readRL, requirePerm("landing_pages:read")).Get("/landing-pages/starter-templates", lpDeps.ListStarterTemplates)
			r.With(readRL, requirePerm("landing_pages:read")).Get("/landing-pages/{id}", lpDeps.GetProject)
			r.With(writeRL, bodySizeMW(bodySizeBatch), requirePerm("landing_pages:update")).Put("/landing-pages/{id}", lpDeps.UpdateProject)
			r.With(writeRL, requirePerm("landing_pages:delete")).Delete("/landing-pages/{id}", lpDeps.DeleteProject)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("landing_pages:create")).Post("/landing-pages/{id}/duplicate", lpDeps.DuplicateProject)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("landing_pages:read")).Post("/landing-pages/{id}/preview", lpDeps.PreviewProject)
			r.With(writeRL, bodySizeMW(bodySizeBatch), requirePerm("landing_pages:update")).Post("/landing-pages/{id}/import", lpDeps.ImportHTML)
			r.With(writeRL, requirePerm("landing_pages:update")).Post("/landing-pages/{id}/import-zip", lpDeps.ImportZIP)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("landing_pages:create")).Post("/landing-pages/{id}/clone-url", lpDeps.CloneURL)

			// Landing page build and hosting endpoints.
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("landing_pages:update")).Post("/landing-pages/{id}/build", lpDeps.TriggerBuild)
			r.With(readRL, requirePerm("landing_pages:read")).Get("/landing-pages/{id}/builds", lpDeps.ListBuilds)
			r.With(readRL, requirePerm("landing_pages:read")).Get("/landing-pages/{id}/builds/{buildId}", lpDeps.GetBuild)
			r.With(writeRL, requirePerm("landing_pages:update")).Post("/landing-pages/{id}/builds/{buildId}/start", lpDeps.StartApp)
			r.With(writeRL, requirePerm("landing_pages:update")).Post("/landing-pages/{id}/builds/{buildId}/stop", lpDeps.StopApp)
			r.With(readRL, requirePerm("landing_pages:read")).Get("/landing-pages/{id}/builds/{buildId}/health", lpDeps.GetAppHealth)

			// Campaign CRUD endpoints (Phase 3 - Campaign Engine).
			r.With(readRL, requirePerm("campaigns:read")).Get("/campaigns", campaignDeps.ListCampaigns)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:create")).Post("/campaigns", campaignDeps.CreateCampaign)
			r.With(readRL, requirePerm("campaigns:read")).Get("/campaigns/{id}", campaignDeps.GetCampaign)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:update")).Put("/campaigns/{id}", campaignDeps.UpdateCampaign)
			r.With(writeRL, requirePerm("campaigns:delete")).Delete("/campaigns/{id}", campaignDeps.DeleteCampaign)

			// Campaign state transition endpoints.
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:update")).Post("/campaigns/{id}/submit", campaignDeps.SubmitForApproval)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:execute")).Post("/campaigns/{id}/build", campaignDeps.BuildCampaign)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:execute")).Post("/campaigns/{id}/launch", campaignDeps.LaunchCampaign)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:execute")).Post("/campaigns/{id}/pause", campaignDeps.PauseCampaign)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:execute")).Post("/campaigns/{id}/resume", campaignDeps.ResumeCampaign)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:execute")).Post("/campaigns/{id}/complete", campaignDeps.CompleteCampaign)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:execute")).Post("/campaigns/{id}/archive", campaignDeps.ArchiveCampaign)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:update")).Post("/campaigns/{id}/unlock", campaignDeps.UnlockCampaignApproval)

			// Campaign approval workflow endpoints.
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:approve")).Post("/campaigns/{id}/approve", campaignDeps.ApproveCampaign)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:approve")).Post("/campaigns/{id}/reject", campaignDeps.RejectCampaign)
			r.With(readRL, requirePerm("campaigns:read")).Get("/campaigns/{id}/approval-review", campaignDeps.GetApprovalReview)
			r.With(readRL, requirePerm("campaigns:read")).Get("/campaigns/{id}/approval-history", campaignDeps.GetApprovalHistory)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:approve")).Post("/campaigns/{id}/blocklist-override", campaignDeps.BlocklistOverrideCampaign)

			// Campaign configuration endpoints.
			r.With(readRL, requirePerm("campaigns:read")).Get("/campaigns/{id}/template-variants", campaignDeps.GetTemplateVariants)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:update")).Put("/campaigns/{id}/template-variants", campaignDeps.SetTemplateVariants)
			r.With(readRL, requirePerm("campaigns:read")).Get("/campaigns/{id}/send-windows", campaignDeps.GetSendWindows)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:update")).Put("/campaigns/{id}/send-windows", campaignDeps.SetSendWindows)

			// Campaign metrics and build logs.
			r.With(readRL, requirePerm("campaigns:read")).Get("/campaigns/{id}/metrics", metricsDeps.GetCampaignMetrics)
			r.With(readRL, requirePerm("campaigns:read")).Get("/campaigns/{id}/metrics/timeline", metricsDeps.GetCampaignTimeline)
			r.With(readRL, requirePerm("campaigns:read")).Get("/campaigns/{id}/build-log", campaignDeps.GetBuildLog)

			// Variant comparison (A/B testing).
			r.With(readRL, requirePerm("campaigns:read")).Get("/campaigns/{id}/variant-comparison", campaignDeps.GetVariantComparison)

			// Canary targets.
			r.With(readRL, requirePerm("campaigns:read")).Get("/campaigns/{id}/canary-targets", campaignDeps.GetCanaryTargets)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:update")).Post("/campaigns/{id}/canary-targets", campaignDeps.SetCanaryTargets)

			// Email delivery status (Phase 3 - Campaign Engine).
			r.With(readRL, requirePerm("campaigns:read")).Get("/campaigns/{id}/delivery-status", emailDeliveryDeps.GetDeliveryStatus)

			// Campaign cloning.
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:create")).Post("/campaigns/{id}/clone", campaignDeps.CloneCampaign)

			// Reusable campaign config templates.
			r.With(readRL, requirePerm("campaigns:read")).Get("/campaign-templates", campaignDeps.ListConfigTemplates)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:create")).Post("/campaign-templates", campaignDeps.CreateConfigTemplate)
			r.With(readRL, requirePerm("campaigns:read")).Get("/campaign-templates/{id}", campaignDeps.GetConfigTemplate)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:update")).Put("/campaign-templates/{id}", campaignDeps.UpdateConfigTemplate)
			r.With(writeRL, requirePerm("campaigns:delete")).Delete("/campaign-templates/{id}", campaignDeps.DeleteConfigTemplate)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:create")).Post("/campaign-templates/{id}/apply", campaignDeps.ApplyConfigTemplate)

			// Endpoint management endpoints (Phase 3 - Campaign Engine).
			r.With(readRL, requirePerm("infrastructure:read")).Get("/campaigns/{id}/endpoint", endpointDeps.GetCampaignEndpoint)
			r.With(readRL, requirePerm("infrastructure:read")).Get("/campaigns/{id}/endpoint/health", endpointDeps.GetCampaignEndpointHealth)
			r.With(readRL, requirePerm("infrastructure:read")).Get("/campaigns/{id}/endpoint/logs", endpointDeps.GetCampaignEndpointLogs)
			r.With(writeRL, requirePerm("infrastructure:update")).Post("/campaigns/{id}/endpoint/stop", endpointDeps.StopCampaignEndpoint)
			r.With(writeRL, requirePerm("infrastructure:update")).Post("/campaigns/{id}/endpoint/restart", endpointDeps.RestartCampaignEndpoint)
			r.With(writeRL, requirePerm("infrastructure:delete")).Delete("/campaigns/{id}/endpoint", endpointDeps.TerminateCampaignEndpoint)
			r.With(writeRL, requirePerm("infrastructure:update")).Post("/campaigns/{id}/endpoint/retry", endpointDeps.RetryCampaignEndpoint)
			r.With(writeRL, requirePerm("infrastructure:update")).Post("/campaigns/{id}/endpoint/redeploy", redeployDeps.RedeployCampaignEndpoint)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("infrastructure:update")).Post("/campaigns/{id}/endpoint/tls", endpointDeps.UploadTLSCertificate)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:update")).Post("/campaigns/{id}/phishing-reports", endpointDeps.ManualPhishingReport)
			r.With(readRL, requirePerm("infrastructure:read")).Get("/endpoints", endpointDeps.ListAllEndpoints)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("infrastructure:create")).Post("/endpoints/provision", provisionDeps.ProvisionEndpoint)

			// Credential capture endpoints (Phase 3 - Campaign Engine).
			r.With(readRL, requirePerm("credentials:read")).Get("/captures", credDeps.ListCaptures)
			r.With(readRL, requirePerm("credentials:read")).Get("/captures/export", credDeps.ExportCaptures)
			r.With(readRL, requirePerm("credentials:read")).Get("/captures/{id}", credDeps.GetCapture)
			r.With(writeRL, requirePerm("credentials:reveal")).Post("/captures/{id}/reveal", credDeps.RevealCapture)
			r.With(writeRL, requirePerm("credentials:delete")).Delete("/captures/{id}", credDeps.DeleteCapture)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("credentials:read")).Post("/captures/{id}/associate", credDeps.AssociateTarget)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("credentials:purge")).Post("/captures/purge", credDeps.PurgeCaptures)
			r.With(readRL, requirePerm("credentials:read")).Get("/campaigns/{id}/captures", credDeps.ListCampaignCaptures)
			r.With(readRL, requirePerm("campaigns:read")).Get("/campaigns/{id}/capture-metrics", credDeps.GetCaptureMetrics)

			// Field categorization endpoints.
			r.With(readRL, requirePerm("credentials:read")).Get("/landing-pages/{id}/field-categories", credDeps.GetCategorizationRules)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("credentials:read")).Post("/landing-pages/{id}/field-categories", credDeps.UpsertCategorizationRule)
			r.With(writeRL, requirePerm("credentials:delete")).Delete("/field-categories/{id}", credDeps.DeleteCategorizationRule)

			// Organization metrics and defender dashboard endpoints.
			r.With(readRL, requirePerm("campaigns:read")).Get("/metrics/organization", metricsDeps.GetOrganizationMetrics)
			r.With(readRL, requirePerm("campaigns:read")).Get("/metrics/departments", metricsDeps.GetDepartmentMetrics)
			r.With(readRL, requirePerm("campaigns:read")).Get("/metrics/trends", metricsDeps.GetTrends)

			// Report generation and management.
			r.With(readRL, requirePerm("campaigns:read")).Get("/report-templates", reportDeps.ListTemplates)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:read")).Post("/report-templates", reportDeps.CreateTemplate)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:read")).Post("/reports/generate", reportDeps.GenerateReport)
			r.With(readRL, requirePerm("campaigns:read")).Get("/reports", reportDeps.ListReports)
			r.With(readRL, requirePerm("campaigns:read")).Get("/reports/{id}", reportDeps.GetReport)
			r.With(readRL, requirePerm("campaigns:read")).Get("/reports/{id}/download", reportDeps.DownloadReport)
			r.With(writeRL, requirePerm("campaigns:delete")).Delete("/reports/{id}", reportDeps.DeleteReport)

			// Notification endpoints (any authenticated user — own data).
			r.With(readRL).Get("/notifications", notifDeps.List)
			r.With(writeRL).Put("/notifications/{id}/read", notifDeps.Read)
			r.With(writeRL).Post("/notifications/read-all", notifDeps.ReadAll)
			r.With(writeRL).Delete("/notifications/{id}", notifDeps.Delete)
			r.With(writeRL).Post("/notifications/delete-read", notifDeps.DeleteRead)
			r.With(writeRL, bodySizeMW(bodySizeStandard)).Post("/notifications/delete-selected", notifDeps.DeleteSelected)
			r.With(readRL).Get("/notifications/unread-count", notifDeps.UnreadCount)
			r.With(readRL).Get("/notifications/preferences", notifDeps.GetPreferences)
			r.With(writeRL, bodySizeMW(bodySizeStandard)).Put("/notifications/preferences", notifDeps.UpdatePreferences)

			// Notification SMTP config (admin only).
			r.With(readRL, requirePerm("settings:read")).Get("/notifications/smtp-config", notifDeps.GetSMTPConfig)
			r.With(writeRL, requirePerm("settings:update"), bodySizeMW(bodySizeStandard)).Put("/notifications/smtp-config", notifDeps.UpsertSMTPConfig)

			// Webhook endpoints.
			r.With(readRL, requirePerm("settings:read")).Get("/webhooks", notifDeps.ListWebhooks)
			r.With(writeRL, requirePerm("settings:update"), bodySizeMW(bodySizeStandard)).Post("/webhooks", notifDeps.CreateWebhook)
			r.With(writeRL, requirePerm("settings:update")).Delete("/webhooks/{id}", notifDeps.DeleteWebhook)
			r.With(writeRL, requirePerm("settings:update")).Put("/webhooks/{id}/toggle", notifDeps.ToggleWebhook)
			r.With(readRL, requirePerm("settings:read")).Get("/webhooks/{id}/deliveries", notifDeps.WebhookDeliveries)
		})

		// AI integration endpoints (all stubs — deferred).
		r.Group(func(r chi.Router) {
			r.Use(requireSetupComplete)
			r.Use(apiKeyAuth)
			r.Use(requireAuth)
			r.Use(middleware.CSRFProtection)

			r.With(readRL, requirePerm("campaigns:read")).Get("/ai/proposals", aiDeps.ListProposals)
			r.With(readRL, requirePerm("campaigns:read")).Get("/ai/proposals/{id}", aiDeps.GetProposal)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:create")).Post("/ai/proposals/{id}/review", aiDeps.ReviewProposal)
			r.With(writeRL, requirePerm("campaigns:delete")).Delete("/ai/proposals/{id}", aiDeps.DeleteProposal)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:create")).Post("/ai/generate/email-template", aiDeps.GenerateEmailTemplate)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:create")).Post("/ai/generate/subject-lines", aiDeps.GenerateSubjectLines)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:create")).Post("/ai/generate/landing-page-content", aiDeps.GenerateLandingPageContent)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:create")).Post("/ai/generate/personalization", aiDeps.GeneratePersonalization)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:create")).Post("/ai/research/target-org", aiDeps.ResearchTargetOrg)
			r.With(writeRL, bodySizeMW(bodySizeStandard), requirePerm("campaigns:create")).Post("/ai/research/industry-templates", aiDeps.ResearchIndustryTemplates)
			r.With(readRL, requirePerm("campaigns:read")).Get("/ai/research/{id}", aiDeps.GetResearch)
			r.With(readRL, requirePerm("campaigns:read")).Get("/ai/research", aiDeps.ListResearch)
		})

		// Internal API — build token auth, localhost only (generated app -> framework).
		r.Route("/internal", func(r chi.Router) {
			r.Use(middleware.RequireBuildToken(lpRepo))
			r.Use(middleware.OptionalEndpointAuth(endpointTokenCache))
			r.Post("/captures", internalDeps.HandleCapture)
			r.Post("/tracking", internalDeps.HandleTracking)
			r.Post("/telemetry", internalDeps.HandleTelemetry)
			r.Post("/delivery-result", internalDeps.HandleDeliveryResult)
			r.Post("/session-captures", internalDeps.HandleSessionCapture)
		})

		// Endpoint data channel — authenticated via X-Build-Token or Bearer token.
		r.Route("/endpoint-data", func(r chi.Router) {
			r.Use(requireEndpointAuth)
			r.Post("/heartbeat", endpointDeps.ReceiveHeartbeat)
			r.Post("/logs", endpointDeps.ReceiveRequestLogs)
		})

		// Phishing report webhook — authenticated via X-Build-Token.
		r.With(requireEndpointAuth).Post("/webhooks/phishing-reports", endpointDeps.ReceivePhishingReport)

		// WebSocket — JWT validated inside handler, outside the requireAuth group.
		r.Get("/ws", notifDeps.WS)
	})

	return r, auditService
}

// bodySizeMW returns middleware that limits the request body to n bytes.
func bodySizeMW(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}
