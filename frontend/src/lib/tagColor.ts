import type { SxProps, Theme } from '@mui/material/styles';
import { darken } from '@mui/material/styles';

/**
 * Chip styling for a tag color: the color as background with readable
 * (black or white) text, slightly darker on hover and focus for clickable
 * or deletable chips. Without a color (a tag not saved yet) the chip keeps
 * its default look.
 */
export function tagChipSx(color: string | undefined): SxProps<Theme> {
  if (!color) return {};

  return (theme) => {
    const text = theme.palette.getContrastText(color);

    return {
      bgcolor: color,
      color: text,
      '&.MuiChip-clickable:hover, &.Mui-focusVisible': { bgcolor: darken(color, 0.15) },
      '& .MuiChip-deleteIcon': { color: text, opacity: 0.7, '&:hover': { color: text, opacity: 1 } },
    };
  };
}
