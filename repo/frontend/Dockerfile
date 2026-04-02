# ── Stage 1: Build ───────────────────────────────────────────────────────────
FROM node:20-alpine AS builder

WORKDIR /app

# Install deps (separate layer for cache)
COPY package.json package-lock.json* ./
RUN if [ -f package-lock.json ]; then \
			npm ci --prefer-offline; \
		else \
			npm install --prefer-offline --no-audit --no-fund; \
		fi

# Copy source and build
COPY . .

# VITE_API_URL is injected at build time via docker-compose environment
ARG VITE_API_URL=http://localhost:8080
ENV VITE_API_URL=$VITE_API_URL

RUN npm run build

# ── Stage 2: Serve with nginx ─────────────────────────────────────────────────
FROM nginx:1.27-alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
