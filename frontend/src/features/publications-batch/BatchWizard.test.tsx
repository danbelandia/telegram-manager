// Smoke tests del BatchWizard (publications-batch change, spec REQ-25
// frontend 6+ casos). Cubre: abrir modal, add/remove slot, submit
// bloqueado si slot invalido, all-success banner, all-fail banner,
// retry de fallidas, summary Alert.
//
// Usa renderWithProviders + mockFetchRoutes (frontend/test/helpers).
// Cero llamadas reales a Telegram o al backend.
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import BatchWizard from './BatchWizard'
import { errorJson, mockFetchRoutes, okJson, renderWithProviders } from '../../test/helpers'

const groups = [
  {
    id: '1',
    telegram_id: -100123,
    title: 'MU Online Comunidad',
    username: null,
    type: 'supergroup',
    member_count: 4821,
    bot_status: 'administrator',
    bot_permissions: {},
  },
  {
    id: '2',
    telegram_id: -100456,
    title: 'Programadores',
    username: null,
    type: 'supergroup',
    member_count: 1203,
    bot_status: 'administrator',
    bot_permissions: {},
  },
]

let batchRespFn: (() => unknown) | null = null
let batchCalls: Array<{ body: string }> = []

beforeEach(() => {
  batchCalls = []
  batchRespFn = null
  mockFetchRoutes({
    '/api/groups': () => okJson(groups),
    '/api/publications/batch': () =>
      batchRespFn
        ? Promise.resolve(batchRespFn())
        : Promise.resolve(okJson({ created: [], failed: [] })),
  })
  // Capturamos el body del POST batch para asserts. Envolvemos el fetch
  // mockeado por mockFetchRoutes para registrar el body enviado.
  const originalFetch = globalThis.fetch
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    if (url.includes('/api/publications/batch') && init?.method === 'POST') {
      batchCalls.push({ body: String(init.body ?? '') })
    }
    return (originalFetch as unknown as (i: RequestInfo | URL, init?: RequestInit) => Promise<unknown>)(input, init)
  }) as typeof fetch
})

afterEach(() => {
  vi.restoreAllMocks()
})

// Helper: llena el slot `i` con texto + grupo valido y dispara submit.
async function fillSlotAndSubmit(user: ReturnType<typeof userEvent.setup>, i: number, text: string, groupLabels: string[]) {
  const slot = await screen.findByTestId(`batch-slot-${i}`)
  const textarea = slot.querySelector('textarea') as HTMLTextAreaElement
  await user.type(textarea, text)
  for (const labelText of groupLabels) {
    // Mantine Checkbox: el texto esta en un <label> adentro del slot;
    // clickeamos ese label (que togglea el input subyacente).
    const allLabels = Array.from(slot.querySelectorAll('label'))
    const target = allLabels.find((l) => l.textContent?.trim() === labelText)
    if (target) await user.click(target)
  }
}

describe('BatchWizard', () => {
  it('abre con 2 slots por default', async () => {
    const onClose = vi.fn()
    renderWithProviders(<BatchWizard opened onClose={onClose} />)
    await waitFor(() => {
      expect(screen.getByTestId('batch-slot-0')).toBeInTheDocument()
      expect(screen.getByTestId('batch-slot-1')).toBeInTheDocument()
    })
    expect(screen.queryByTestId('batch-slot-2')).not.toBeInTheDocument()
  })

  it('agregar slot aumenta el contador hasta el maximo; remover lo baja', async () => {
    const user = userEvent.setup()
    renderWithProviders(<BatchWizard opened onClose={vi.fn()} />)

    const addBtn = screen.getByTestId('batch-add-slot')
    // Default 2 slots. Agregamos hasta llegar al cap (10). Probamos el
    // boundary directamente: agregamos 8 para llegar a 10, removemos
    // uno y verificamos que el add vuelva a habilitarse. Para no
    // impactar el timeout del test, agregamos slots suficientes hasta
    // ver el disabled y luego removemos uno solo.
    for (let i = 0; i < 8; i++) {
      if (addBtn.hasAttribute('disabled')) break
      await user.click(addBtn)
    }
    // Llegamos al maximo (10) -> add debe estar disabled.
    expect(addBtn).toBeDisabled()
    expect(screen.getByTestId('batch-slot-9')).toBeInTheDocument()

    // Remover uno vuelve a habilitar el add.
    await user.click(screen.getByTestId('batch-remove-slot-9'))
    expect(screen.queryByTestId('batch-slot-9')).not.toBeInTheDocument()
    expect(addBtn).not.toBeDisabled()
  }, 15000)

  it('submit deshabilitado si algun slot tiene text vacio o 0 grupos', async () => {
    const user = userEvent.setup()
    renderWithProviders(<BatchWizard opened onClose={vi.fn()} />)

    // Llenamos solo el slot 0.
    const slot0 = screen.getByTestId('batch-slot-0')
    const textarea0 = slot0.querySelector('textarea') as HTMLTextAreaElement
    await user.type(textarea0, 'hola')
    // Click en grupo MU Online.
    const slot0Labels = Array.from(slot0.querySelectorAll('label'))
    const slot0Group = slot0Labels.find((l) => l.textContent?.trim() === 'MU Online Comunidad')
    if (slot0Group) await user.click(slot0Group)

    // Slot 1 sigue vacio -> submit deshabilitado.
    const submit = screen.getByTestId('batch-submit') as HTMLButtonElement
    expect(submit).toBeDisabled()

    // Llenamos slot 1 tambien.
    const slot1 = screen.getByTestId('batch-slot-1')
    const textarea1 = slot1.querySelector('textarea') as HTMLTextAreaElement
    await user.type(textarea1, 'mundo')
    const slot1Labels = Array.from(slot1.querySelectorAll('label'))
    const slot1Group = slot1Labels.find((l) => l.textContent?.trim() === 'MU Online Comunidad')
    if (slot1Group) await user.click(slot1Group)

    await waitFor(() => {
      expect(submit).not.toBeDisabled()
    })
  })

  it('response all-success muestra solo banner verde + boton Cerrar', async () => {
    batchRespFn = () =>
      okJson({
        created: [
          { index: 0, publication: { id: 1, telegram_id: -100123, text: 'a', status: 'sent', message_id: 1, error_message: null, actor_id: 1, photo_url: null, buttons: null, scheduled_at: null, created_at: '2026-09-08T10:00:00Z' } },
          { index: 1, publication: { id: 2, telegram_id: -100456, text: 'b', status: 'sent', message_id: 2, error_message: null, actor_id: 1, photo_url: null, buttons: null, scheduled_at: null, created_at: '2026-09-08T10:00:00Z' } },
        ],
        failed: [],
      })

    const user = userEvent.setup()
    renderWithProviders(<BatchWizard opened onClose={vi.fn()} />)

    // Llenar slot 0 y slot 1.
    await fillSlotAndSubmit(user, 0, 'hola 1', ['MU Online Comunidad'])
    await fillSlotAndSubmit(user, 1, 'hola 2', ['Programadores'])

    const submit = screen.getByTestId('batch-submit')
    await user.click(submit)

    await waitFor(() => {
      expect(screen.getByTestId('batch-success-alert')).toBeInTheDocument()
    })
    expect(screen.queryByTestId('batch-failure-alert')).not.toBeInTheDocument()
    expect(screen.getByTestId('batch-close')).toBeInTheDocument()
    expect(screen.queryByTestId('batch-retry')).not.toBeInTheDocument()
  })

  it('response all-fail muestra solo banner rojo + Reintentar + Cerrar', async () => {
    batchRespFn = () =>
      okJson({
        created: [],
        failed: [
          { index: 0, code: 'VALIDATION_ERROR', message: 'texto no puede estar vacio' },
          { index: 1, code: 'PERMISSION_DENIED', message: 'el bot no tiene permisos suficientes' },
        ],
      })

    const user = userEvent.setup()
    renderWithProviders(<BatchWizard opened onClose={vi.fn()} />)

    await fillSlotAndSubmit(user, 0, 'a', ['MU Online Comunidad'])
    await fillSlotAndSubmit(user, 1, 'b', ['Programadores'])

    await user.click(screen.getByTestId('batch-submit'))

    await waitFor(() => {
      expect(screen.getByTestId('batch-failure-alert')).toBeInTheDocument()
    })
    expect(screen.queryByTestId('batch-success-alert')).not.toBeInTheDocument()
    expect(screen.getByTestId('batch-retry')).toBeInTheDocument()
    expect(screen.getByTestId('batch-close')).toBeInTheDocument()
  })

  it('response parcial muestra ambos banners', async () => {
    batchRespFn = () =>
      okJson({
        created: [
          { index: 0, publication: { id: 1, telegram_id: -100123, text: 'a', status: 'sent', message_id: 1, error_message: null, actor_id: 1, photo_url: null, buttons: null, scheduled_at: null, created_at: '2026-09-08T10:00:00Z' } },
        ],
        failed: [
          { index: 1, code: 'VALIDATION_ERROR', message: 'se requiere al menos un grupo' },
        ],
      })

    const user = userEvent.setup()
    renderWithProviders(<BatchWizard opened onClose={vi.fn()} />)

    await fillSlotAndSubmit(user, 0, 'a', ['MU Online Comunidad'])
    await fillSlotAndSubmit(user, 1, 'b', ['Programadores'])

    await user.click(screen.getByTestId('batch-submit'))

    await waitFor(() => {
      expect(screen.getByTestId('batch-success-alert')).toBeInTheDocument()
      expect(screen.getByTestId('batch-failure-alert')).toBeInTheDocument()
    })
    expect(screen.getByTestId('batch-retry')).toBeInTheDocument()
  })

  it('Reintentar fallidas filtra slots a solo failed[] y re-envia', async () => {
    let firstCall = true
    batchRespFn = () => {
      if (firstCall) {
        firstCall = false
        return okJson({
          created: [
            { index: 0, publication: { id: 1, telegram_id: -100123, text: 'a', status: 'sent', message_id: 1, error_message: null, actor_id: 1, photo_url: null, buttons: null, scheduled_at: null, created_at: '2026-09-08T10:00:00Z' } },
          ],
          failed: [
            { index: 1, code: 'VALIDATION_ERROR', message: 'reintentar' },
          ],
        })
      }
      // Segundo intento: todo success.
      return okJson({
        created: [
          { index: 0, publication: { id: 2, telegram_id: -100456, text: 'b', status: 'sent', message_id: 2, error_message: null, actor_id: 1, photo_url: null, buttons: null, scheduled_at: null, created_at: '2026-09-08T10:01:00Z' } },
        ],
        failed: [],
      })
    }

    const user = userEvent.setup()
    renderWithProviders(<BatchWizard opened onClose={vi.fn()} />)

    await fillSlotAndSubmit(user, 0, 'a', ['MU Online Comunidad'])
    await fillSlotAndSubmit(user, 1, 'b', ['Programadores'])

    await user.click(screen.getByTestId('batch-submit'))

    await waitFor(() => {
      expect(screen.getByTestId('batch-failure-alert')).toBeInTheDocument()
    })

    // Click "Reintentar fallidas" -> solo el slot con index en failed[]
    // (index=1) queda. Como queda 1 slot, pasa a ser slot 0.
    await user.click(screen.getByTestId('batch-retry'))

    await waitFor(() => {
      // Solo 1 slot presente (el que fallo).
      expect(screen.getByTestId('batch-slot-0')).toBeInTheDocument()
      expect(screen.queryByTestId('batch-slot-1')).not.toBeInTheDocument()
    })

    // Confirmar que el texto pre-rellenado sobrevivio al retry.
    const slot0After = screen.getByTestId('batch-slot-0')
    const textareaAfter = slot0After.querySelector('textarea') as HTMLTextAreaElement
    expect(textareaAfter.value).toBe('b')

    await user.click(screen.getByTestId('batch-submit'))

    await waitFor(() => {
      // Segundo intento -> solo verde.
      expect(screen.getByTestId('batch-success-alert')).toBeInTheDocument()
    })
    expect(batchCalls.length).toBeGreaterThanOrEqual(2)
    // El segundo body tiene solo 1 publicacion.
    const lastBody = JSON.parse(batchCalls[batchCalls.length - 1].body) as { publications: unknown[] }
    expect(lastBody.publications.length).toBe(1)
  })

  it('Summary Alert muestra cada slot con su target y fecha antes del submit', async () => {
    const user = userEvent.setup()
    renderWithProviders(<BatchWizard opened onClose={vi.fn()} />)

    await fillSlotAndSubmit(user, 0, 'bienvenidos', ['MU Online Comunidad'])
    await fillSlotAndSubmit(user, 1, 'hola devs', ['Programadores'])

    const summary = screen.getByTestId('batch-summary')
    expect(summary.textContent).toMatch(/bienvenidos/)
    expect(summary.textContent).toMatch(/hola devs/)
    expect(summary.textContent).toMatch(/MU Online Comunidad/)
    expect(summary.textContent).toMatch(/Programadores/)
  })

  it('error HTTP del batch muestra el Alert rojo y permite reintentar', async () => {
    batchRespFn = () => errorJson(400, 'VALIDATION_ERROR', 'maximo 10 publicaciones por batch')

    const user = userEvent.setup()
    renderWithProviders(<BatchWizard opened onClose={vi.fn()} />)

    await fillSlotAndSubmit(user, 0, 'a', ['MU Online Comunidad'])
    await fillSlotAndSubmit(user, 1, 'b', ['Programadores'])

    await user.click(screen.getByTestId('batch-submit'))

    await waitFor(() => {
      expect(screen.getByTestId('batch-create-error')).toBeInTheDocument()
    })
  })
})
