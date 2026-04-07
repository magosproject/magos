FROM node:20-alpine AS build-stage

WORKDIR /app
COPY package.json package-lock.json ./

RUN npm ci

COPY . .

RUN npm run build

FROM nginx:alpine

COPY nginx.conf.template /etc/nginx/templates/nginx.conf.template
COPY --from=build-stage /app/build/client /usr/share/nginx/html

ENV MAGOS_API_HOST=localhost
ENV MAGOS_API_PORT=8080

EXPOSE 8080

USER nginx

CMD ["sh", "-c", "envsubst '${MAGOS_API_HOST} ${MAGOS_API_PORT}' < /etc/nginx/templates/nginx.conf.template > /tmp/nginx.conf && nginx -c /tmp/nginx.conf -g 'daemon off;'"]
