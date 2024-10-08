# syntax=docker/dockerfile:1
# Keep this syntax directive! It's used to enable Docker BuildKit

################################
# PYTHON-BASE
# Sets up all our shared environment variables
################################
FROM python:3.11-slim AS python-base

    # Python
ENV PYTHONUNBUFFERED=1 \
    # pip
    PIP_DISABLE_PIP_VERSION_CHECK=on \
    PIP_DEFAULT_TIMEOUT=100 \
    \
    # Poetry
    # https://python-poetry.org/docs/configuration/#using-environment-variables
    POETRY_VERSION=1.8.3 \
    # make poetry install to this location
    POETRY_HOME="/opt/poetry" \
    # do not ask any interactive question
    POETRY_NO_INTERACTION=1 \
    # never create virtual environment automaticly, only use env prepared by us
    POETRY_VIRTUALENVS_CREATE=false \
    \
    # this is where our requirements + virtual environment will live
    VIRTUAL_ENV="/venv" \
    \
    # Node.js major version. Remove if you don't need.
    NODE_MAJOR=20

# prepend poetry and venv to path
ENV PATH="$POETRY_HOME/bin:$VIRTUAL_ENV/bin:$PATH"

# prepare virtual env
RUN python -m venv $VIRTUAL_ENV

# working directory and Python path
WORKDIR /app
ENV PYTHONPATH="/app"

# pretrained models cache path. Remove if you don't need.
# ref: https://huggingface.co/docs/transformers/installation?highlight=transformers_cache#caching-models
ENV TRANSFORMERS_CACHE="/opt/transformers_cache/"

################################
# BUILDER-BASE
# Used to build deps + create our virtual environment
################################
FROM python-base AS builder-base
RUN apt-get update && \
    apt-get install -y \
    apt-transport-https \
    build-essential \
    ca-certificates \
    curl \
    git \
    gnupg \
    nano

# install poetry - respects $POETRY_VERSION & $POETRY_HOME
# The --mount will mount the buildx cache directory to where
# Poetry and Pip store their cache so that they can re-use it
RUN --mount=type=cache,target=/root/.cache \
    curl -sSL https://install.python-poetry.org | python -

# used to init dependencies
WORKDIR /app
COPY poetry.lock pyproject.toml ./
# install runtime deps to $VIRTUAL_ENV
RUN poetry export -f requirements.txt > requirements.txt && \
    python -m pip install -r requirements.txt

#COPY scripts scripts/
#COPY src rule

# populate Huggingface model cache. Remove if you don't need.
#RUN poetry run python scripts/bootstrap.py

# build C dependencies. Remove if you don't need.
#RUN --mount=type=cache,target=/app/scripts/vendor \
#    poetry run python scripts/build-c-denpendencies.py && \
#    cp scripts/lib/*.so /usr/lib

#################################
## DEVELOPMENT
## Image used during development / testing
#################################
#FROM builder-base AS development
#
#WORKDIR /app
#
## quicker install AS runtime deps are already installed
#RUN --mount=type=cache,target=/root/.cache \
#    poetry install --no-root --with test,lint
#
#EXPOSE 8080
#CMD ["bash"]


################################
# PRODUCTION
# Final image used for runtime
################################
FROM python-base AS production

RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates && \
    apt-get clean

# copy in our built poetry + venv
COPY --from=builder-base $POETRY_HOME $POETRY_HOME
COPY --from=builder-base $VIRTUAL_ENV $VIRTUAL_ENV
# copy in our C dependencies. Remove if you don't need.
#COPY --from=builder-base /app/scripts/lib/*.so /usr/lib
# copy in pre-populated transformer cache. Remove if you don't need.
#COPY --from=builder-base $TRANSFORMERS_CACHE $TRANSFORMERS_CACHE

WORKDIR /app
COPY poetry.lock pyproject.toml ./
COPY src rule

EXPOSE 8000
CMD ["python", "/app/rule_service/main.py"]
