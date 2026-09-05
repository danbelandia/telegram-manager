# Imagen de desarrollo: Vite dev server con hot reload.
# En compose se monta ./frontend:/app y un volumen anonimo para
# node_modules (friccion OneDrive/Windows con bind mounts).
FROM node:20-alpine

WORKDIR /app

# Solo cambia cuando package.json / package-lock.json cambian
COPY package.json package-lock.json* ./
RUN npm install

COPY . .

EXPOSE 5173
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]