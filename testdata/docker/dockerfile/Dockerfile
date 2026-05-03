FROM ubuntu:22.04
WORKDIR /app
COPY . .
RUN apt-get update && apt-get install -y --no-install-recommends curl \
  && rm -rf /var/lib/apt/lists/*
CMD ["./start.sh"]
