import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, request, setAccessToken, setOnUnauthorized } from './api-client'

describe('api-client', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    setAccessToken(null)
    setOnUnauthorized(null)
  })

  const envelope = (data: unknown) => JSON.stringify({ data, error: null })
  const errorEnvelope = (code: string, message: string) =>
    JSON.stringify({ data: null, error: { code, message } })

  it('agrega Authorization Bearer cuando hay access token', async () => {
    setAccessToken('token-123')
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve(JSON.parse(envelope([]))),
    })
    vi.stubGlobal('fetch', fetchMock)

    await request('/api/groups')

    const init = fetchMock.mock.calls[0][1] as RequestInit
    expect(init.headers).toMatchObject({ Authorization: 'Bearer token-123' })
  })

  it('normaliza el envelope y devuelve data', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve(JSON.parse(envelope([{ id: 1 }]))),
    })
    vi.stubGlobal('fetch', fetchMock)

    const data = await request<{ id: number }[]>('/api/groups')
    expect(data).toEqual([{ id: 1 }])
  })

  it('lanza ApiError con code y mensaje del envelope', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: false,
      status: 502,
      json: () => Promise.resolve(JSON.parse(errorEnvelope('TELEGRAM_ERROR', 'telegram fallo'))),
    })
    vi.stubGlobal('fetch', fetchMock)

    const err = await request('/api/groups/1/users/2/ban', { method: 'POST' }).catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).code).toBe('TELEGRAM_ERROR')
    expect((err as ApiError).status).toBe(502)
  })

  it('ante 401 refresca con la cookie y reejecuta una sola vez', async () => {
    const fetchMock = vi
      .fn()
      // 1) request original -> 401 UNAUTHORIZED
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: () => Promise.resolve(JSON.parse(errorEnvelope('UNAUTHORIZED', 'token expirado'))),
      })
      // 2) refresh -> nuevo access token
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve(JSON.parse(envelope({ access_token: 'nuevo-token' }))),
      })
      // 3) reejecucion -> 200
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve(JSON.parse(envelope([{ id: 7 }]))),
      })
    vi.stubGlobal('fetch', fetchMock)

    const data = await request<{ id: number }[]>('/api/groups')

    expect(data).toEqual([{ id: 7 }])
    expect(fetchMock).toHaveBeenCalledTimes(3)
    const refreshCall = fetchMock.mock.calls[1]
    expect(String(refreshCall[1].method)).toBe('POST')
    expect(String(fetchMock.mock.calls[0][0])).toContain('/api/groups')
  })

  it('si el refresh falla, invoca onUnauthorized y no reejecuta', async () => {
    const onUnauth = vi.fn()
    setOnUnauthorized(onUnauth)
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: () => Promise.resolve(JSON.parse(errorEnvelope('UNAUTHORIZED', 'token expirado'))),
      })
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: () => Promise.resolve(JSON.parse(errorEnvelope('UNAUTHORIZED', 'refresh invalido'))),
      })
    vi.stubGlobal('fetch', fetchMock)

    await expect(request('/api/groups')).rejects.toBeInstanceOf(ApiError)

    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(onUnauth).toHaveBeenCalledTimes(1)
  })

  it('logout sin body responde 204 y devuelve undefined', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
      json: () => Promise.reject(new Error('204 no tiene body')),
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(request('/api/auth/logout', { method: 'POST' })).resolves.toBeUndefined()
  })
})