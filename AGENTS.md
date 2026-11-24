## General conventions
- Use Docker Compose to set up the different services.
- Use the .env.skel file to contain the skeleton of the ENV file, and put there all the environment variables needed, with a comment regarding what they are
used for. Put them into sections.
- Be creative and don't hesitate to research online design principles of modern web design and software architecture. Do not use conventional design that looks AI-made.
- Use a simple MariaDB database for data storage

## Backend Conventions
- Use Go Lang

## Frontend Conventions
- Use Svelte framework

## Website color palette

Use this palette for brand accents and readable text on white or light backgrounds.

| Color | Hex | Contrast on white | Recommended use |
|---|---|---:|---|
| Rosy Taupe | `#9b514d` | 5.71:1 | Warm accent text, links, icons, and secondary buttons. |
| Old Rose | `#8e4168` | 6.76:1 | Secondary links, selected navigation, badges, and chart accents. |
| Amethyst Smoke | `#783d78` | 7.67:1 | Primary brand links, focus indicators, and prominent accents. |
| Dusty Lavender | `#68466f` | 7.79:1 | Primary buttons, emphasized controls, and section headings. |
| Space Indigo | `#3b3b58` | 10.76:1 | Main headings, high-emphasis text, navigation, and footer accents. |

### Accessibility and usage rules

- These palette colors meet WCAG AA's 4.5:1 contrast requirement for normal-sized text when used on white (`#ffffff`).
- Use white text on these colors for buttons or dark accent panels; each color also has at least 4.5:1 contrast with white.
- Recheck contrast if a palette color is placed on a tinted surface, image, gradient, or another palette color. Contrast depends on the actual foreground and background pair.
- Use color alongside text, shape, or icon changes to show status; do not make color the only signal.
- Treat these as brand colors, not semantic status colors. Use a separate accessible palette for success, warning, and error states.
- Preserve visible keyboard focus indicators and verify their contrast against adjacent surfaces.
- Use neutral backgrounds, such as white or a light gray, to keep the page calm and let the accent colors stand out.
