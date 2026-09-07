// Editor controlado de botones inline (slice 2): array de filas, cada
// fila un array de {text, url}. Expone agregar/quitar fila y
// agregar/quitar boton por fila. Limites cliente: 8 filas, 8 botones
// por fila, text<=64, url http(s) — la validacion final vive en el
// servicio, este componente solo evita que el cliente envie algo
// trivialmente invalido.
import type { InlineButton } from './types'

interface ButtonsEditorProps {
  value: InlineButton[][]
  onChange: (rows: InlineButton[][]) => void
}

function emptyButton(): InlineButton {
  return { text: '', url: '' }
}

export default function ButtonsEditor({ value, onChange }: ButtonsEditorProps) {
  const addRow = () => {
    if (value.length >= 8) return
    onChange([...value, [emptyButton()]])
  }
  const removeRow = (rowIdx: number) => {
    onChange(value.filter((_, i) => i !== rowIdx))
  }
  const addButton = (rowIdx: number) => {
    if (value[rowIdx].length >= 8) return
    onChange(value.map((row, i) => (i === rowIdx ? [...row, emptyButton()] : row)))
  }
  const removeButton = (rowIdx: number, btnIdx: number) => {
    onChange(
      value.map((row, i) =>
        i === rowIdx ? row.filter((_, j) => j !== btnIdx) : row,
      ),
    )
  }
  const updateButton = (
    rowIdx: number,
    btnIdx: number,
    field: 'text' | 'url',
    next: string,
  ) => {
    onChange(
      value.map((row, i) =>
        i === rowIdx
          ? row.map((btn, j) => (j === btnIdx ? { ...btn, [field]: next } : btn))
          : row,
      ),
    )
  }

  return (
    <div className="buttons-editor">
      <p className="state-block-weak">Botones inline (URL): máximo 8 filas × 8 botones.</p>
      {value.length === 0 ? (
        <button type="button" className="btn" onClick={addRow}>
          + Agregar fila de botones
        </button>
      ) : (
        <>
          {value.map((row, rowIdx) => (
            <div key={rowIdx} className="buttons-editor-row">
              <div className="buttons-editor-row-header">
                <strong>Fila {rowIdx + 1}</strong>
                <button
                  type="button"
                  className="btn btn-danger"
                  onClick={() => removeRow(rowIdx)}
                  aria-label={`Quitar fila ${rowIdx + 1}`}
                >
                  Quitar fila
                </button>
              </div>
              {row.map((btn, btnIdx) => (
                <div key={btnIdx} className="buttons-editor-btn">
                  <input
                    type="text"
                    placeholder="Texto del botón"
                    value={btn.text}
                    maxLength={64}
                    onChange={(e) => updateButton(rowIdx, btnIdx, 'text', e.target.value)}
                    aria-label={`Texto del botón ${rowIdx + 1}.${btnIdx + 1}`}
                  />
                  <input
                    type="url"
                    placeholder="https://..."
                    value={btn.url}
                    maxLength={256}
                    onChange={(e) => updateButton(rowIdx, btnIdx, 'url', e.target.value)}
                    aria-label={`URL del botón ${rowIdx + 1}.${btnIdx + 1}`}
                  />
                  <button
                    type="button"
                    className="btn btn-danger"
                    onClick={() => removeButton(rowIdx, btnIdx)}
                    aria-label={`Quitar botón ${rowIdx + 1}.${btnIdx + 1}`}
                  >
                    ×
                  </button>
                </div>
              ))}
              <button
                type="button"
                className="btn"
                onClick={() => addButton(rowIdx)}
                disabled={row.length >= 8}
              >
                + Agregar botón a la fila
              </button>
            </div>
          ))}
          <button type="button" className="btn" onClick={addRow} disabled={value.length >= 8}>
            + Agregar fila de botones
          </button>
        </>
      )}
    </div>
  )
}
