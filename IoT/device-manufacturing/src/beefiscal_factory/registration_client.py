from __future__ import annotations
import httpx

class RegistrationClient:
    def __init__(self, base_url: str, oidc_token: str) -> None:
        if not base_url.startswith("https://") and "localhost" not in base_url:
            raise ValueError("production registration endpoint must use HTTPS")
        if not oidc_token:
            raise ValueError("OIDC workload token required")
        self.base_url = base_url.rstrip("/")
        self.headers = {"Authorization": f"Bearer {oidc_token}", "Content-Type": "application/json"}

    def register(self, payload: dict) -> dict:
        response = httpx.post(f"{self.base_url}/platform/v1/manufacturing/devices:register", headers=self.headers, json=payload, timeout=15)
        response.raise_for_status()
        return response.json()
