import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Markdown } from './Markdown';

interface MarkdownEditorProps {
  id: string;
  value: string;
  onChange: (value: string) => void;
}

/** A textarea with a Write / Preview toggle. */
export function MarkdownEditor({ id, value, onChange }: MarkdownEditorProps) {
  const { t } = useTranslation();
  const [preview, setPreview] = useState(false);

  return (
    <div className="md-editor">
      <div className="tabs" role="tablist">
        <button type="button" role="tab" aria-selected={!preview} onClick={() => setPreview(false)}>
          {t('editor.write')}
        </button>
        <button type="button" role="tab" aria-selected={preview} onClick={() => setPreview(true)}>
          {t('editor.preview')}
        </button>
      </div>
      {preview ? (
        <div className="md-preview">{value.trim() ? <Markdown>{value}</Markdown> : <p className="muted">{t('editor.nothing')}</p>}</div>
      ) : (
        <textarea id={id} rows={10} value={value} onChange={(e) => onChange(e.target.value)} />
      )}
      <small className="hint">{t('editor.markdownHint')}</small>
    </div>
  );
}
