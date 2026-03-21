FROM node:22-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
ARG VITE_EVIDRA_API_URL=https://api.evidra.cc
ARG VITE_EVIDRA_API_KEY
ENV VITE_EVIDRA_API_URL=${VITE_EVIDRA_API_URL}
ENV VITE_EVIDRA_API_KEY=${VITE_EVIDRA_API_KEY}
RUN npm run build

FROM nginx:alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
