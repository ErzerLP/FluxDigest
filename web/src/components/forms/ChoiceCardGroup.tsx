interface ChoiceCardOption<TValue extends string> {
  value: TValue;
  label: string;
  description: string;
}

interface ChoiceCardGroupProps<TValue extends string> {
  name: string;
  label: string;
  value: TValue;
  options: Array<ChoiceCardOption<TValue>>;
  onChange: (value: TValue) => void;
}

export function ChoiceCardGroup<TValue extends string>({
  name,
  label,
  value,
  options,
  onChange,
}: ChoiceCardGroupProps<TValue>) {
  return (
    <fieldset className="form-fieldset form-stack">
      <legend>{label}</legend>
      <div className="choice-card-grid" role="radiogroup" aria-label={label}>
        {options.map((option) => {
          const checked = value === option.value;

          return (
            <label
              key={option.value}
              className={checked ? 'choice-card choice-card-active' : 'choice-card'}
            >
              <input
                className="sr-only"
                type="radio"
                name={name}
                aria-label={`${label} / ${option.label}`}
                checked={checked}
                onChange={() => onChange(option.value)}
              />
              <span className="choice-card-title">{option.label}</span>
              <span className="choice-card-description">{option.description}</span>
            </label>
          );
        })}
      </div>
    </fieldset>
  );
}
