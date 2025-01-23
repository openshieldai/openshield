FROM python:3.10-slim


WORKDIR /app


RUN apt-get update && apt-get install --no-install-recommends -y \
      automake autoconf build-essential libtool libc-dev make flex gcc \
      pkg-config libssl-dev curl unzip git git-lfs bison \
      && rm -rf /var/lib/apt/lists/*


RUN echo "Installing YARA from source ..." \
      && curl -Lo yara.zip https://github.com/VirusTotal/yara/archive/refs/tags/v4.3.2.zip \
      && unzip yara.zip \
      && cd yara-4.3.2 \
      && ./bootstrap.sh \
      && ./configure \
      && make \
      && make install \
      && make check \
      && cd .. \
      && rm -rf yara.zip yara-4.3.2


RUN git clone https://github.com/deadbits/vigil-llm.git .


COPY packages.txt .


RUN echo "Installing Python dependencies ... " \
      && pip install --no-cache-dir -r packages.txt


COPY generate_config.sh /app/generate_config.sh
RUN chmod +x /app/generate_config.sh



EXPOSE 5000


COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh


ENTRYPOINT ["/entrypoint.sh", "python", "-m", "vigil.vigil", "-c", "conf/server.conf"]
