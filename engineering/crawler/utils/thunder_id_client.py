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

    # by caching the access token in memory will improve the performance. otherwise everytime get access token is called, another network call will be introduced even we have the valid access token.
    # there is no any error handling inside this network call which leads to crash the server if the 'access_token' isnt exsits in the response.
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
            # DO NOT print the access token in the log
            logger.info("Access token: %s", token)
            return token