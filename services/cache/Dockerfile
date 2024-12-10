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
    # this is where our requirements + virtual environment will live
    VIRTUAL_ENV="/venv"

# prepend poetry and venv to path
ENV PATH="$POETRY_HOME/bin:$VIRTUAL_ENV/bin:$PATH"

# prepare virtual env
RUN python -m venv "$VIRTUAL_ENV"

# working directory and Python path
WORKDIR /app
ENV PYTHONPATH="/app"

ENV HF_HOME="/app/transformers_cache/"

################################
# BUILDER-BASE
# Used to build deps + create our virtual environment
################################
FROM python-base AS builder-base
RUN apt-get update && \
    apt-get install --no-install-recommends -y apt-transport-https \
    build-essential \
    ca-certificates \
    curl \
    git \
    gnupg && \
    apt-get clean

# install poetry - respects $POETRY_VERSION & $POETRY_HOME
# The --mount will mount the buildx cache directory to where
# Poetry and Pip store their cache so that they can re-use it
RUN curl --proto "=https" --tlsv1.2 -sSf -L https://install.python-poetry.org | python -

# used to init dependencies
WORKDIR /app
COPY poetry.lock pyproject.toml ./
# install runtime deps to $VIRTUAL_ENV
RUN poetry export -f requirements.txt > requirements.txt && \
    python -m pip install -r requirements.txt

################################
# PRODUCTION
# Final image used for runtime
#################################
FROM python-base AS production

RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates && \
    apt-get clean && \
    rm -rf /app && \
    usermod -d /app -m nobody && \
    mkdir -p /app/cache && \
    chown -R nobody:nogroup /app

WORKDIR /app
USER nobody:nogroup
# copy in our built poetry + venv
COPY --from=builder-base --chmod=755 --chown=nobody:nogroup $POETRY_HOME $POETRY_HOME
COPY --from=builder-base --chmod=755 --chown=nobody:nogroup $VIRTUAL_ENV $VIRTUAL_ENV

COPY --chmod=755 --chown=nobody:nogroup poetry.lock pyproject.toml ./
COPY --chmod=755 --chown=nobody:nogroup main.py ./

EXPOSE 8080
CMD ["python","main.py"]
