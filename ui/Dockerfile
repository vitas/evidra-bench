FROM node:22-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
ARG VITE_BENCH_API_URL=
ARG VITE_BASE_PATH=/
ENV VITE_BENCH_API_URL=${VITE_BENCH_API_URL}
ENV VITE_BASE_PATH=${VITE_BASE_PATH}
RUN npm run build

FROM nginx:alpine
ARG VERSION=dev
ARG COMMIT=dev
ARG BUILD_DATE=unknown
LABEL org.opencontainers.image.title="Evidra Bench UI" \
      org.opencontainers.image.description="Public reports and dashboard UI for Evidra Bench" \
      org.opencontainers.image.source="https://github.com/vitas/evidra-bench" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}"
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
