FROM nginx:1.27-alpine

COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY frontend/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
