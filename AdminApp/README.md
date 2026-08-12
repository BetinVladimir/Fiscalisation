# BeeFiscal AdminApp

Platform device inventory and tenant assignment UI. Authentication is only OIDC
Authorization Code + PKCE with `beefiscal.platform`; access tokens remain in
process memory and no privileged token environment fallback exists.
