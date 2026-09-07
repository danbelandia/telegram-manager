// PostCSS config requerida por Mantine v7 para `light-dark()` y los
// breakpoints (`$sm`, `$md`, etc.). Sin esto, los tokens del theme no
// compilan correctamente (ver design.md D3 — frontend-refresh slice 1).
module.exports = {
  plugins: {
    'postcss-preset-mantine': {},
    'postcss-simple-vars': {
      variables: {
        'mantine-breakpoint-xs': '36em',
        'mantine-breakpoint-sm': '48em',
        'mantine-breakpoint-md': '62em',
        'mantine-breakpoint-lg': '75em',
        'mantine-breakpoint-xl': '88em',
      },
    },
  },
}