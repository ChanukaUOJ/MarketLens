import os
 
BACKEND_BASE_URL = os.getenv("BACKEND_URL", "http://backend:8080/api/v1/crawler")
BACKEND_URL_FOR_FETCHING = os.getenv("BACKEND_URL_FOR_FETCHING", "http://backend:8080/api/v1")
DEEPSEEK_API_KEY = os.getenv("DEEPSEEK_API_KEY")
BATCH_SIZE = 10

THUNDER_BASE_URL = os.getenv("THUNDER_BASE_URL")
THUNDER_CLIENT_ID = os.getenv("THUNDER_CLIENT_ID")
THUNDER_CLIENT_SECRET = os.getenv("THUNDER_CLIENT_SECRET")
THUNDER_RESOURCE = os.getenv("THUNDER_RESOURCE")
# there is no any fallback value for this which leads to an error when try to perform .lower() on a None value
THUNDER_VERIFY_TLS = os.getenv("THUNDER_INSECURE_TLS").lower() != "true"