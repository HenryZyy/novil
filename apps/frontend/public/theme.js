// Sets the saved theme before the page draws, so it doesn't flash the
// other one. A file, not inline: the Content-Security-Policy allows no
// inline scripts.
try {
  const theme = localStorage.getItem('novil.theme')
  if (theme === 'light' || theme === 'dark') document.documentElement.dataset.theme = theme
} catch {}
