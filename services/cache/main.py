import argparse
import logging
import os
from typing import Optional, Dict
from urllib.parse import urlparse

from gptcache import Cache
from gptcache.adapter.api import (
    get,
    put,
    init_similar_cache_from_config,
)
from gptcache.session import Session
from gptcache.utils import import_fastapi, import_pydantic, import_starlette, import_sqlalchemy

import_fastapi()
import_pydantic()
import_sqlalchemy()

from fastapi import FastAPI, HTTPException
import uvicorn
from pydantic import BaseModel

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)
app = FastAPI()
openai_caches: Dict[str, Cache] = {}
redis_url = os.getenv("REDIS_URL")
postgres_url = os.getenv("DATABASE_URL")


def parse_redis_url(url):
    parsed = urlparse(url)
    return {
        'host': parsed.hostname or 'localhost',
        'port': parsed.port or 6379,
        'db': int(parsed.path.lstrip('/') or 0),
        'password': parsed.password if parsed.password else None,
        'ssl': True if parsed.scheme == 'rediss' else False
    }


class CacheData(BaseModel):
    prompt: str
    answer: Optional[str] = ""
    product_id: str


def openshield_check_hit_func(cur_session_id, cache_session_ids, cache_questions, cache_answer):
    return cur_session_id in cache_session_ids


@app.post("/put")
async def put_cache(cache_data: CacheData) -> str:
    session = Session(name=cache_data.product_id, check_hit_func=openshield_check_hit_func)
    put(cache_data.prompt, cache_data.answer, session=session)
    logger.info(f"Setting cache data: %s", cache_data.prompt)
    return "successfully update the cache"


@app.post("/get")
async def get_cache(cache_data: CacheData) -> CacheData:
    session = Session(name=cache_data.product_id, check_hit_func=openshield_check_hit_func)
    logger.info(f"Getting cache data: %s", cache_data.prompt)
    result = get(cache_data.prompt, session=session)

    if result is None:
        logger.info(f"Cache miss for prompt: %s", cache_data.prompt)
        raise HTTPException(status_code=404, detail="Cache miss")
    else:
        logger.info(f"Cache hit for prompt: %s", cache_data.prompt)
    return CacheData(prompt=cache_data.prompt, answer=result, product_id=cache_data.product_id)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "-s", "--host", default="0.0.0.0", help="the hostname to listen on"
    )
    parser.add_argument(
        "-p", "--port", type=int, default=8080, help="the port to listen on"
    )
    parser.add_argument(
        "-d", "--cache-dir", default="gptcache_data", help="the cache data dir"
    )
    parser.add_argument("-k", "--cache-file-key", default="", help="the cache file key")
    parser.add_argument(
        "-f", "--cache-config-file", default="config.yaml", help="the cache config file"
    )
    parser.add_argument(
        "-o",
        "--openai",
        type=bool,
        default=False,
        help="whether to open the openai completes proxy",
    )
    parser.add_argument(
        "-of",
        "--openai-cache-config-file",
        default=None,
        help="the cache config file of the openai completes proxy",
    )

    args = parser.parse_args()

    if args.cache_config_file:
        init_conf = init_similar_cache_from_config(config_dir=args.cache_config_file)
        init_conf.get("storage_config", {}).get("data_dir", "")

    if args.openai and args.openai_cache_config_file:
        init_similar_cache_from_config(
            config_dir=args.openai_cache_config_file,
        )

    import_starlette()
    from starlette.middleware.cors import CORSMiddleware

    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    uvicorn.run(app, host=args.host, port=args.port)


if __name__ == "__main__":
    main()
