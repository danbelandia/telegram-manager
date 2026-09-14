// Landing publica `/` — navbar sticky con anchors a 3 secciones
// (#about, #features, #pricing) + hero + Quiénes somos + Qué hace
// + Precios + footer. Los anchors del navbar usan `<a href="#x">` plano
// para que el browser haga el scroll por anchor nativo (sin que React
// Router intercepte como ruta). Las secciones tienen scroll-margin-top
// para que el navbar sticky (60px) no tape el titulo. Layout de
// features/pricing inspirado en Mantine HeroBullets: titulos centrados,
// listas con check icons, dos botones en el hero (Crear cuenta /
// Iniciar sesion). Responsive: en mobile apila verticalmente y oculta
// la imagen del hero.
import {
  Anchor,
  Box,
  Button,
  Container,
  Group,
  Image,
  List,
  SimpleGrid,
  Stack,
  Text,
  ThemeIcon,
  Title,
} from '@mantine/core'
import { IconCheck } from '@tabler/icons-react'
import { Link } from 'react-router-dom'
import heroSvg from './image.35935aae.svg'
import classes from './LandingPage.module.css'

const NAV_LINKS: ReadonlyArray<{ to: string; label: string }> = [
  { to: '#about', label: 'Quiénes somos' },
  { to: '#features', label: 'Qué hace' },
  { to: '#pricing', label: 'Precios' },
]

export default function LandingPage() {
  return (
    <>
      {/* Navbar sticky con anchors a las secciones */}
      <Box
        component="nav"
        className={classes.navbar}
        data-testid="landing-navbar"
      >
        <Container size="lg" className={classes.navbarInner}>
          <Group justify="space-between" align="center" w="100%">
            <Title order={4} fw={700} className={classes.brand}>
              Sentinel
            </Title>
            <Group gap="lg" visibleFrom="xs">
              {NAV_LINKS.map((link) => (
                <Anchor
                  key={link.to}
                  href={link.to}
                  className={classes.navLink}
                  underline="never"
                  data-testid={`landing-nav-${link.to.replace('#', '')}`}
                >
                  {link.label}
                </Anchor>
              ))}
            </Group>
          </Group>
        </Container>
      </Box>

      {/* Hero */}
      <Container size="lg">
        <div className={classes.inner}>
          <div className={classes.content}>
            <Title className={classes.title}>
              Administrá tus grupos de{' '}
              <span className={classes.highlight}>Telegram</span>
            </Title>

            <Text className={classes.description} c="dimmed" mt="md">
              Un solo panel para moderar usuarios, aprobar solicitudes de ingreso,
              programar publicaciones y auditar cada acción administrativa.
            </Text>

            <List
              className={classes.featureList}
              spacing="sm"
              size="md"
              icon={
                <ThemeIcon size={24} radius="xl" variant="light" color="blue">
                  <IconCheck size={14} stroke={2} />
                </ThemeIcon>
              }
            >
              <List.Item>
                <b>Moderación con un clic</b> — banear, mutear, expulsar
                usuarios y eliminar mensajes desde el panel.
              </List.Item>
              <List.Item>
                <b>Solicitudes de ingreso</b> — aceptá o rechazá nuevos
                miembros sin abrir Telegram.
              </List.Item>
              <List.Item>
                <b>Publicaciones programadas</b> — escribí el mensaje, elegí
                la fecha y dejá que el bot lo publique por vos.
              </List.Item>
              <List.Item>
                <b>Auditoría completa</b> — cada acción queda registrada con
                quién la hizo, cuándo y el resultado.
              </List.Item>
            </List>

            <Stack className={classes.controls} gap="sm">
              <Button
                component={Link}
                to="/signup"
                radius="xl"
                size="lg"
                variant="filled"
              >
                Crear cuenta gratis
              </Button>
              <Button
                component={Link}
                to="/login"
                radius="xl"
                size="lg"
                variant="default"
              >
                Iniciar sesión
              </Button>
            </Stack>
          </div>

          <Image
            src={heroSvg}
            className={classes.heroImage}
            alt="Sentinel"
            w={340}
            h={340}
          />
        </div>
      </Container>

      {/* Sección 1: Quiénes somos */}
      <Box id="about" className={classes.section}>
        <Container size="md">
          <Stack gap="md">
            <Title order={2} ta="center">
              Quiénes somos
            </Title>
            <Text size="lg" c="dimmed" ta="center" maw={720} mx="auto">
              Sentinel es un equipo chico de desarrolladores y moderadores que
              vive la comunidad de Telegram desde adentro. Construimos esta
              plataforma porque administrar grupos grandes a mano se vuelve
              insostenible: las solicitudes se acumulan, el spam aparece y los
              logs se pierden. Nuestra misión es devolverles a los dueños de
              comunidades el control y el tiempo, con herramientas serias y
              respetuosas de la privacidad.
            </Text>
          </Stack>
        </Container>
      </Box>

      {/* Sección 2: Qué hace nuestro producto */}
      <Box id="features" className={classes.section}>
        <Container size="lg">
          <Stack gap="lg">
            <Title order={2} ta="center">
              Qué hace nuestro producto
            </Title>
            <Text size="lg" c="dimmed" ta="center" maw={720} mx="auto">
              Todo lo que necesitás para operar tus grupos de Telegram sin abrir
              la app.
            </Text>

            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="xl" mt="xl">
              <List
                spacing="sm"
                size="md"
                icon={
                  <ThemeIcon size={24} radius="xl" variant="light" color="blue">
                    <IconCheck size={14} stroke={2} />
                  </ThemeIcon>
                }
              >
                <List.Item>
                  <b>Moderación con un clic</b> — banear, mutear, expulsar
                  usuarios y eliminar mensajes desde el panel.
                </List.Item>
                <List.Item>
                  <b>Solicitudes de ingreso</b> — aceptá o rechazá nuevos
                  miembros sin abrir Telegram.
                </List.Item>
                <List.Item>
                  <b>Publicaciones programadas</b> — escribí el mensaje, elegí
                  la fecha y dejá que el bot lo publique por vos.
                </List.Item>
                <List.Item>
                  <b>Auditoría completa</b> — cada acción queda registrada con
                  quién la hizo, cuándo y el resultado.
                </List.Item>
              </List>

              <List
                spacing="sm"
                size="md"
                icon={
                  <ThemeIcon size={24} radius="xl" variant="light" color="blue">
                    <IconCheck size={14} stroke={2} />
                  </ThemeIcon>
                }
              >
                <List.Item>
                  <b>Apertura y cierre del chat</b> — restringí el envío de
                  mensajes cuando tu grupo lo necesite.
                </List.Item>
                <List.Item>
                  <b>Fijar mensajes</b> — anclá anuncios importantes al tope del
                  grupo con un click.
                </List.Item>
                <List.Item>
                  <b>Multi-grupo</b> — administrá varias comunidades desde un
                  solo panel, con permisos por tenant.
                </List.Item>
                <List.Item>
                  <b>Rate limiting inteligente</b> — el bot respeta los límites
                  de la API de Telegram sin que tengas que pensarlo.
                </List.Item>
              </List>
            </SimpleGrid>
          </Stack>
        </Container>
      </Box>

      {/* Sección 3: Precios */}
      <Box id="pricing" className={classes.section}>
        <Container size="md">
          <Stack gap="lg">
            <Title order={2} ta="center">
              Precios
            </Title>
            <Text size="lg" c="dimmed" ta="center" maw={720} mx="auto">
              Empezá gratis con todo lo que necesitás para un grupo mediano.
            </Text>

            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="xl" mt="xl">
              <Box className={classes.pricingCard} data-testid="pricing-trial">
                <Stack gap="sm">
                  <Title order={3}>Trial</Title>
                  <Text size="xs" c="dimmed">
                    Prueba todas las funciones por 3 días, sin tarjeta.
                  </Text>
                  <Text size="xl" fw={700}>
                    Trial 3 días
                  </Text>
                  <Text c="dimmed" size="sm">
                    Acceso completo a Sentinel durante 3 días para probar todo sin compromiso.
                  </Text>
                  <List
                    spacing="xs"
                    size="sm"
                    icon={
                      <ThemeIcon
                        size={18}
                        radius="xl"
                        variant="light"
                        color="blue"
                      >
                        <IconCheck size={12} stroke={2} />
                      </ThemeIcon>
                    }
                  >
                    <List.Item>Acceso completo por 3 días</List.Item>
                    <List.Item>Todos los grupos habilitados</List.Item>
                    <List.Item>Moderación manual completa</List.Item>
                    <List.Item>Sin tarjeta de crédito</List.Item>
                  </List>
                  <Button
                    component={Link}
                    to="/signup"
                    radius="md"
                    variant="filled"
                    mt="md"
                  >
                    Empezar gratis
                  </Button>
                </Stack>
              </Box>

              <Box className={classes.pricingCard} data-testid="pricing-pro">
                <Stack gap="sm">
                  <Group gap="xs" align="center">
                    <Title order={3}>Pro</Title>
                    <Text size="xs" c="blue" fw={700} tt="uppercase">
                      Próximamente
                    </Text>
                  </Group>
                  <Text size="xl" fw={700}>
                    $6.99 / mes
                  </Text>
                  <Text c="dimmed" size="sm">
                    Para comunidades grandes y equipos de moderación. Facturación mensual, cancelás cuando quieras.
                  </Text>
                  <List
                    spacing="xs"
                    size="sm"
                    icon={
                      <ThemeIcon
                        size={18}
                        radius="xl"
                        variant="light"
                        color="blue"
                      >
                        <IconCheck size={12} stroke={2} />
                      </ThemeIcon>
                    }
                  >
                    <List.Item>Grupos ilimitados</List.Item>
                    <List.Item>Publicaciones programadas</List.Item>
                    <List.Item>Moderación automática</List.Item>
                    <List.Item>Soporte prioritario</List.Item>
                    <List.Item>Sin límite de tiempo</List.Item>
                  </List>
                  <Button
                    radius="md"
                    variant="default"
                    mt="md"
                    disabled
                    data-testid="pricing-pro-cta"
                  >
                    Próximamente
                  </Button>
                </Stack>
              </Box>
            </SimpleGrid>
          </Stack>
        </Container>
      </Box>

      {/* Footer */}
      <Box
        component="footer"
        className={classes.footer}
        data-testid="landing-footer"
      >
        <Container size="lg">
          <Group justify="space-between" align="center" wrap="wrap" gap="md">
            <Text size="sm" c="dimmed">
              © 2026 Sentinel
            </Text>
            <Text size="sm" c="dimmed">
              Plataforma para administración de grupos de Telegram
            </Text>
            <Group gap="md">
              <Anchor component={Link} to="/terms" c="dimmed" size="sm" underline="hover">
                Términos
              </Anchor>
              <Anchor component={Link} to="/privacy" c="dimmed" size="sm" underline="hover">
                Privacidad
              </Anchor>
            </Group>
          </Group>
        </Container>
      </Box>
    </>
  )
}
