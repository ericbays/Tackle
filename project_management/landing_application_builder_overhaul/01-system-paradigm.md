# 01 — Landing Application Paradigm & Dynamic Nature

## 1. Core Intent and Infrastructure
The Landing Application is the standalone product resulting from the Landing Application Builder within the Tackle framework. It is dynamically constructed by red team operators to handle a vast array of phishing campaign techniques.

- **Hosting Layer:** Tackle hosts the Landing Application on an arbitrary port on the same server that runs Tackle.
- **Ingress Layer:** Separate servers, known as Phishing Endpoints, are deployed by Tackle. These endpoints act as the public face that targets hit, transparently forwarding all target traffic upstream to the Landing Application.

## 2. Dual Responsibility
The generated application inherently balances two primary responsibilities:
1. **Dynamic Execution:** Operating the varied and highly configurable campaign techniques designed by the operator (routing, rendering, credential capture, interaction flows).
2. **Internal Telemetry:** Statically maintaining a stealth telemetry and event pipeline. It tracks target interactions, captures events, and internally forwards upstream metrics back to the Tackle server framework.

## 3. Signature Evasion (Anti-Fingerprinting)
The most critical architectural constraint of the Landing Application is **Signature Evasion**. 
Code reuse between campaigns is a liability. If a defensive team successfully signatures a Landing Application and configures static tools to block it, the next Landing Application built by Tackle must produce an entirely different compiled footprint.
- The "offensive code" (both frontend logic and Go backend structure, routing, and behavior) must be procedurally varied.
- The generation pipeline cannot rely on static boilerplates; the resulting applications must be structurally distinct per-campaign to permanently defeat static fingerprinting mechanisms.
