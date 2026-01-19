/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    screens: {
      xs: '320px',
      sm: '480px',
      md: '768px',
      lg: '1024px',
      xl: '1280px',
      '2xl': '1536px',
    },
    extend: {
      fontSize: {
        xs: ['12px', '16px'],
        sm: ['14px', '20px'],
        base: ['16px', '24px'],
        lg: ['18px', '28px'],
        xl: ['20px', '28px'],
        '2xl': ['24px', '32px'],
        '3xl': ['30px', '36px'],
        '4xl': ['36px', '40px'],
        '5xl': ['48px', '52px'],
        '6xl': ['60px', '64px'],
        '7xl': ['72px', '80px'],
        '8xl': ['96px', '100px'],
      },
      spacing: {
        xs: '0.25rem',
        sm: '0.5rem',
        md: '1rem',
        lg: '1.5rem',
        xl: '2rem',
        '2xl': '2.5rem',
        '3xl': '3rem',
      },
      colors: {
        primary: {
          light: '#ffffff',
          dark: '#121212',
        },
        secondary: {
          light: '#f5f5f5',
          dark: '#1f1f1f',
        },
        tertiary: {
          light: '#eeeeee',
          dark: '#2a2a2a',
        },
        text: {
          primary: {
            light: '#000000',
            dark: '#ffffff',
          },
          secondary: {
            light: '#666666',
            dark: '#9CA3AF',
          },
        },
        border: {
          light: '#e0e0e0',
          dark: '#2a2a2a',
        },
        accent: {
          light: '#007AFF',
          dark: '#515BD4',
        },
      },
      animation: {
        pulse: 'pulse 2s ease-in-out infinite',
        fadeIn: 'fadeIn 0.4s ease-out',
        slideIn: 'slideIn 0.3s ease-out',
        slideUp: 'slideUp 0.3s ease-out',
        bounce: 'bounce 0.6s ease-in-out',
        glow: 'glow 2s ease-in-out infinite',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        slideIn: {
          '0%': { transform: 'translateX(-10px)', opacity: '0' },
          '100%': { transform: 'translateX(0)', opacity: '1' },
        },
        slideUp: {
          '0%': { transform: 'translateY(10px)', opacity: '0' },
          '100%': { transform: 'translateY(0)', opacity: '1' },
        },
        bounce: {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-4px)' },
        },
        glow: {
          '0%, 100%': { boxShadow: '0 0 8px rgba(81, 91, 212, 0.2)' },
          '50%': { boxShadow: '0 0 12px rgba(81, 91, 212, 0.4)' },
        },
      },
    },
  },
  plugins: [],
}
