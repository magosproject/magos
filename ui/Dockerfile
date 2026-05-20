FROM node:20-alpine AS build-stage

WORKDIR /app
COPY package.json package-lock.json ./

RUN --mount=type=cache,target=/root/.npm \
    npm ci --prefer-offline

COPY . .

RUN npm run build

FROM nginx:alpine

COPY nginx.conf.template /etc/nginx/templates/nginx.conf.template
COPY --from=build-stage /app/build/client /usr/share/nginx/html

ENV MAGOS_API_HOST=localhost
ENV MAGOS_API_PORT=8080

EXPOSE 8080

USER nginx

COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh
CMD ["/docker-entrypoint.sh"]
