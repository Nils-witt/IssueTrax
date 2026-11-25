import { useTranslation } from 'react-i18next';
import type { IssueStatus } from '../api/types';

export function StatusBadge({ status }: { status: IssueStatus }) {
  const { t } = useTranslation();

  return <span className={`badge status-${status}`}>{t(`status.${status}`)}</span>;
}
