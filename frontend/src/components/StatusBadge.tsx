import Chip, { type ChipProps } from '@mui/material/Chip';
import { useTranslation } from 'react-i18next';
import type { IssueStatus } from '../api/types';

const colors: Record<IssueStatus, ChipProps['color']> = {
  draft: 'default',
  open: 'success',
  closed: 'secondary',
  archived: 'warning',
};

export function StatusBadge({ status }: { status: IssueStatus }) {
  const { t } = useTranslation();

  return <Chip size="small" variant="outlined" color={colors[status]} label={t(`status.${status}`)} sx={{ fontWeight: 600 }} />;
}
