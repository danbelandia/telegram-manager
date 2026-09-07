# Imagen de desarrollo: Vite dev server con hot reload.
# El bind-mount de compose es SOLO ./frontend/src:/app/src (ver
# docker-compose.yml). Por eso el CMD re-corre `npm install` al arrancar:
# la imagen trae node_modules de la build original, pero cualquier cambio
# posterior de package.json/lock del host queda visible al contenedor y
# se reconcilia en el siguiente arranque. Trade-off: +10-30s al primer
# arranque (consistente con el start_period=10s del healthcheck backend).
FROM node:20-alpine

WORKDIR /app

# Solo cambia cuando package.json / package-lock.json cambian
COPY package.json package-lock.json* ./
RUN npm install

COPY . .

EXPOSE 5173
# sh -c ejecuta npm install y luego arranca Vite. Si falla el install
# (deps incompatibles), Vite no arranca y el contenedor queda reiniciando
# (visible en `docker compose logs frontend`), evitando "Cannot find
# module" silencioso en Windows/OneDrive.
CMD ["sh", "-c", "npm install && npm run dev -- --host 0.0.0.0"]