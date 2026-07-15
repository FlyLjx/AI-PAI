FROM node:24-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
COPY apps/web/package.json ./apps/web/package.json
COPY apps/admin/package.json ./apps/admin/package.json
RUN npm ci

FROM node:24-alpine AS build
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build:web

FROM node:24-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0
RUN addgroup -S aipai && adduser -S aipai -G aipai
COPY --from=build --chown=aipai:aipai /app/apps/web/.next/standalone ./
COPY --from=build --chown=aipai:aipai /app/apps/web/.next/static ./apps/web/.next/static
COPY --from=build --chown=aipai:aipai /app/apps/web/public ./apps/web/public
USER aipai
EXPOSE 3000
CMD ["node", "apps/web/server.js"]
