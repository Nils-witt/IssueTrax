import type { SxProps, Theme } from '@mui/material/styles';
import { alpha, darken, lighten } from '@mui/material/styles';

/**
 * Chip styling for a tag color, GitHub-label style: a translucent tint of the
 * color as background, a thin border and the color itself (darkened in light
 * mode, lightened in dark mode) as text, so any color stays readable. Without
 * a color (a tag not saved yet) the chip gets a neutral outline.
 */
export function tagChipSx(color: string | undefined): SxProps<Theme> {
  return (theme) => {
    const base = {
      height: 22,
      fontWeight: 500,
      border: '1px solid',
      '& .MuiChip-label': { px: 1 },
    };

    if (!color) {
      return { ...base, bgcolor: 'transparent', borderColor: theme.vars?.palette.divider ?? theme.palette.divider };
    }

    const light = darken(color, 0.45);
    const dark = lighten(color, 0.4);

    return {
      ...base,
      bgcolor: alpha(color, 0.18),
      color: light,
      borderColor: alpha(light, 0.3),
      '&.MuiChip-clickable:hover, &.Mui-focusVisible': { bgcolor: alpha(color, 0.3) },
      '& .MuiChip-deleteIcon': { color: 'inherit', opacity: 0.7, '&:hover': { color: 'inherit', opacity: 1 } },
      ...theme.applyStyles('dark', { color: dark, borderColor: alpha(dark, 0.3) }),
    };
  };
}
