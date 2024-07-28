FROM node:21.6.2-alpine AS cssbuild

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm install

COPY . .
RUN npm run build

FROM golang:1.22-alpine as gobuild
WORKDIR /app

COPY . .
COPY --from=cssbuild /app/static/app.css /app/static/app.css
RUN go build -o /bin/buzerator .

FROM alpine
WORKDIR /app

RUN apk --no-cache add tzdata
COPY --from=gobuild /bin/buzerator /bin/buzerator
ENV TZ=Europe/Bratislava

CMD ["/bin/buzerator"]
