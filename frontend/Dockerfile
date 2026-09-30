# Build stage.
FROM node:22-alpine AS build

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

# NEXT_PUBLIC_* values are inlined into the bundle at build time, so the ones the
# browser could need have to be present here. The API URL is not among them: it is
# read from API_URL at runtime, which is what lets the same image talk to
# http://api:8080 in compose and to a real host in a deployment.
ARG NEXT_PUBLIC_PAYMENT_LINK_BASE=https://pay.example.com/r
ARG NEXT_PUBLIC_ENABLE_WEBHOOK_SIMULATOR=true
ENV NEXT_PUBLIC_PAYMENT_LINK_BASE=${NEXT_PUBLIC_PAYMENT_LINK_BASE}
ENV NEXT_PUBLIC_ENABLE_WEBHOOK_SIMULATOR=${NEXT_PUBLIC_ENABLE_WEBHOOK_SIMULATOR}
ENV NEXT_TELEMETRY_DISABLED=1

RUN npm run build

# Runtime stage: Next's standalone output carries only the server and the modules it
# actually imports, so node_modules does not ship.
FROM node:22-alpine AS runtime

WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

RUN addgroup -g 10001 -S app && adduser -u 10001 -S app -G app

COPY --from=build --chown=app:app /app/.next/standalone ./
COPY --from=build --chown=app:app /app/.next/static ./.next/static
COPY --from=build --chown=app:app /app/public ./public

USER app
EXPOSE 3000

CMD ["node", "server.js"]
