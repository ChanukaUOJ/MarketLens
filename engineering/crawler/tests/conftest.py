import pytest

from crawlers.base_crawler import BaseJobCrawler


class _ConcreteCrawler(BaseJobCrawler):
    """BaseJobCrawler is abstract; _flush_batch is what we're testing and
    doesn't need a real crawl_jobs implementation behind it."""

    async def crawl_jobs(self, crawler_run_id, async_client):
        raise NotImplementedError


@pytest.fixture
def crawler():
    return _ConcreteCrawler()


@pytest.fixture
def auth_headers():
    return {"Authorization": "Bearer test-token"}
