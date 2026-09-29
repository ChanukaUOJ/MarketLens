import logging

import httpx

from config import (
    THUNDER_BASE_URL,
    THUNDER_CLIENT_ID,
    THUNDER_CLIENT_SECRET,
    THUNDER_RESOURCE,
    THUNDER_VERIFY_TLS,
)

logger = logging.getLogger(__name__)


class ThunderIDClient:
    async def get_access_token(self):
        async with httpx.AsyncClient(verify=THUNDER_VERIFY_TLS) as client:
            response = await client.post(
                f"{THUNDER_BASE_URL}/oauth2/token",
                auth=(THUNDER_CLIENT_ID, THUNDER_CLIENT_SECRET),
                headers={"Content-Type": "application/x-www-form-urlencoded"},
                data={
                    "grant_type": "client_credentials",
                    "scope": "crawler:runs crawler:complete crawler:lookup crawler:batch-save crawler:batch-update crawler:reconcile",
                    "resource": THUNDER_RESOURCE,
                },
            )
            token = response.json()["access_token"]
            logger.info("Access token: %s", token)
            return token
