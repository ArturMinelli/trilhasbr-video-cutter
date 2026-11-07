/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      boxShadow: {
        panel: '0 12px 32px -18px rgba(3, 10, 22, 0.9)',
      },
    },
  },
  plugins: [],
}
