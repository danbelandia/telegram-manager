// Tests de LandingPage (spec frontend-routing REQ public-landing): la
// ruta `/` renderiza la presentacion con navbar sticky, hero con
// enlaces a /signup y /login, 3 secciones (#about, #features, #pricing)
// y footer. La navbar expone anchors a las secciones via React Router
// <Link to="#x"> para que el browser haga el scroll.
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { MantineProvider } from '@mantine/core'
import { describe, expect, it } from 'vitest'
import LandingPage from './LandingPage'
import { mantineTheme } from '../theme'

function renderLanding() {
  return render(
    <MantineProvider theme={mantineTheme} defaultColorScheme="light">
      <MemoryRouter initialEntries={['/']}>
        <LandingPage />
      </MemoryRouter>
    </MantineProvider>,
  )
}

describe('LandingPage', () => {
  it('muestra enlaces a /signup y /login', () => {
    renderLanding()

    expect(
      screen.getByRole('link', { name: 'Crear cuenta gratis' }),
    ).toHaveAttribute('href', '/signup')
    expect(
      screen.getByRole('link', { name: 'Iniciar sesión' }),
    ).toHaveAttribute('href', '/login')
  })

  it('navbar: renderiza los 3 anchors a las secciones', () => {
    renderLanding()

    expect(screen.getByTestId('landing-navbar')).toBeInTheDocument()
    // React Router v6 normaliza <Link to="#x"> a `pathname + "#x"`
    // (en el landing = "/#x"), asi que matcheamos por hash, no por
    // igualdad exacta.
    expect(screen.getByTestId('landing-nav-about').getAttribute('href')).toMatch(/#about$/)
    expect(
      screen.getByTestId('landing-nav-features').getAttribute('href'),
    ).toMatch(/#features$/)
    expect(
      screen.getByTestId('landing-nav-pricing').getAttribute('href'),
    ).toMatch(/#pricing$/)
  })

  it('hero: renderiza "Crear cuenta gratis"', () => {
    renderLanding()

    expect(
      screen.getByRole('link', { name: 'Crear cuenta gratis' }),
    ).toBeInTheDocument()
  })

  it('sections: renderiza about, features y pricing con sus ids', () => {
    renderLanding()

    expect(document.getElementById('about')).toBeInTheDocument()
    expect(document.getElementById('features')).toBeInTheDocument()
    expect(document.getElementById('pricing')).toBeInTheDocument()
  })

  it('footer: copyright, terminos y privacidad', () => {
    renderLanding()

    expect(screen.getByTestId('landing-footer')).toBeInTheDocument()
    expect(screen.getByText('© 2026 Sentinel')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Términos' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Privacidad' })).toBeInTheDocument()
  })
})
