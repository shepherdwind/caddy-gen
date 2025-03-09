FROM oven/bun:1 AS builder
WORKDIR /app
ADD package.json index.ts bun.lock /app
RUN bun i -p

FROM oven/bun:1
WORKDIR /app
ADD package.json index.ts bun.lock /app
COPY --from=builder /app/node_modules /app/node_modules
CMD ["run", "index.ts"]
