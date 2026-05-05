FROM node:22-alpine AS builder

WORKDIR /src

COPY frontend/package.json frontend/package-lock.json ./frontend/
WORKDIR /src/frontend
ENV NPM_CONFIG_REGISTRY=https://registry.npmmirror.com
RUN npm ci

COPY frontend/ ./

ARG VITE_API_BASE_URL=
ARG VITE_REGISTRATION_ENABLED=true
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
ENV VITE_REGISTRATION_ENABLED=${VITE_REGISTRATION_ENABLED}

RUN npm run build

FROM nginx:1.27-alpine

COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /src/frontend/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
