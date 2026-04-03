// Package cloudcredentials provides HTTP handlers for cloud credential and instance template management.
package cloudcredentials

import (
	credsvc "tackle/internal/services/cloudcredential"
	tmplsvc "tackle/internal/services/instancetemplate"
)

// Deps holds the service dependencies for cloud credential and instance template handlers.
type Deps struct {
	CredSvc *credsvc.Service
	TmplSvc *tmplsvc.Service
}
