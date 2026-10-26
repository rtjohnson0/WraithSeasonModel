// Small line icons for the quick-read badges. All drawn on a 24px grid with currentColor.
const PATHS = {
  'weak-defense': <path d="M12 3l7 3v5c0 4.5-3 8-7 10-4-2-7-5.5-7-10V6l7-3zM12 8l-2 4h3l-2 4" />,
  'tough-defense': <path d="M12 3l7 3v5c0 4.5-3 8-7 10-4-2-7-5.5-7-10V6l7-3zM9 12l2 2 4-4" />,
  hot: <path d="M12 3c1 3 5 5 5 10a5 5 0 0 1-10 0c0-2 1-3 2-4 0 2 1 3 2 3 0-3-1-6 1-9z" />,
  cold: <path d="M12 3v18M4.5 7.5l15 9M19.5 7.5l-15 9M9.5 4.5L12 7l2.5-2.5M9.5 19.5L12 17l2.5 2.5" />,
  'trend-for': <path d="M4 17l5-5 4 4 7-8M15 8h5v5" />,
  'trend-against': <path d="M4 7l5 5 4-4 7 8M15 16h5v-5" />,
  'minutes-up': <path d="M12 21a9 9 0 1 1 0-18 9 9 0 0 1 0 18zM12 7v5l3 2M17 3l3 0 0 3" />,
  'minutes-down': <path d="M12 21a9 9 0 1 1 0-18 9 9 0 0 1 0 18zM12 7v5l3 2M17 21l3 0 0-3" />,
  'back-to-back': <path d="M5 8h6v10H5zM13 6h6v10h-6z" />,
  'teammates-out': <path d="M9 11a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM3 20c0-3.5 2.5-6 6-6s6 2.5 6 6M17 7l4 4M21 7l-4 4" />,
  status: <path d="M12 3l10 18H2L12 3zM12 10v5M12 18v.5" />,
  'opponent-out': <path d="M9 11a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM3 20c0-3.5 2.5-6 6-6s6 2.5 6 6M16 5l5 5M21 5l-5 5" />,
  'line-move': <path d="M4 8h12l-3-3M20 16H8l3 3" />,
  'odd-line': <path d="M4 12h4l2-6 4 12 2-6h4" />,
}

export default function Icon({ name }) {
  return (
    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {PATHS[name] || <circle cx="12" cy="12" r="4" />}
    </svg>
  )
}
