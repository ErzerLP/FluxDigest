import type { SecretInput, SecretView } from '../../types/admin';

interface SecretFieldProps {
  id: string;
  label: string;
  value: SecretInput;
  currentSecret?: SecretView;
  onChange: (next: SecretInput) => void;
}

const options = [
  { mode: 'keep', label: '保留现有' },
  { mode: 'replace', label: '替换密钥' },
  { mode: 'clear', label: '清空密钥' },
] as const;

export function SecretField({ id, label, value, currentSecret, onChange }: SecretFieldProps) {
  const effectiveValue = value.mode === 'replace' ? value.value ?? '' : '';

  return (
    <fieldset className="form-fieldset secret-field">
      <legend>{label}</legend>
      <p className="field-hint">
        当前状态：{currentSecret?.is_set ? currentSecret.masked_value ?? '已设置' : '未设置'}
      </p>
      <div className="segmented-control" role="radiogroup" aria-label={label}>
        {options.map((option) => {
          const checked = value.mode === option.mode;
          return (
            <label
              key={option.mode}
              className={checked ? 'segment-option segment-option-active' : 'segment-option'}
            >
              <input
                className="sr-only"
                type="radio"
                name={`${id}-mode`}
                checked={checked}
                onChange={() =>
                  onChange({
                    mode: option.mode,
                    value: option.mode === 'replace' ? effectiveValue : undefined,
                  })
                }
              />
              <span>{option.label}</span>
            </label>
          );
        })}
      </div>

      {value.mode === 'replace' ? (
        <div className="form-stack compact-stack">
          <label className="form-label" htmlFor={id}>
            新 API Key
          </label>
          <input
            id={id}
            className="console-input"
            type="password"
            value={effectiveValue}
            placeholder="sk-..."
            autoComplete="off"
            onChange={(event) => onChange({ mode: 'replace', value: event.target.value })}
          />
        </div>
      ) : null}
    </fieldset>
  );
}
