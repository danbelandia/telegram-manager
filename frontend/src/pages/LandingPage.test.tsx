// Tests de LandingPage (spec frontend-routing REQ public-landing): la
// ruta `/` renderiza la presentacion con enlaces a /signup y /login.
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

    expect(screen.getByRole('link', { name: 'Crear cuenta gratis' })).toHaveAttribute('href', '/signup')
    expect(screen.getByRole('link', { name: 'Iniciar sesión' })).toHaveAttribute('href', '/login')
  })
})
