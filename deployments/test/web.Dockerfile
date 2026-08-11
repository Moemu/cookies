FROM node:24-alpine AS build

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

COPY index.html tsconfig.app.json tsconfig.json tsconfig.node.json vite.config.ts ./
COPY public/ ./public/
COPY src/ ./src/
COPY scripts/check-frontend-bundle-budget.mjs ./scripts/check-frontend-bundle-budget.mjs
RUN npm run build

FROM nginx:1.29-alpine

COPY deployments/test/nginx.conf /etc/nginx/nginx.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
