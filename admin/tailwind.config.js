/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './src/pages/**/*.{js,ts,jsx,tsx,mdx}',
    './src/components/**/*.{js,ts,jsx,tsx,mdx}',
    './src/app/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  theme: {
    extend: {
      colors: {
        primary: '#2563eb',
        secondary: '#0f172a',
        success: '#16a34a',
        warning: '#d97706',
        error: '#dc2626',
        info: '#0284c7',
        slate: {
          25: '#fcfcfd',
        },
      },
    },
  },
  plugins: [],
};
