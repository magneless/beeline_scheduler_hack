# syntax=docker/dockerfile:1

FROM node:22-alpine AS base
WORKDIR /app
RUN corepack enable

FROM base AS deps
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
ENV HUSKY=0
RUN pnpm install --frozen-lockfile --ignore-scripts

FROM deps AS dev
COPY . .
EXPOSE 3000
CMD ["pnpm", "dev", "--host", "0.0.0.0", "--port", "3000"]

FROM deps AS build
COPY . .
ARG VITE_API_MODE=mock
ARG VITE_API_URL=/api/v1
ARG VITE_YANDEX_MAPS_KEY=
ENV VITE_API_MODE=$VITE_API_MODE \
    VITE_API_URL=$VITE_API_URL \
    VITE_YANDEX_MAPS_KEY=$VITE_YANDEX_MAPS_KEY
RUN pnpm build

FROM nginx:1.27-alpine AS preview
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
