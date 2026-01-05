# execbox-cloud Dashboard

Dashboard frontend for execbox-cloud built with Vite, React, TypeScript, and shadcn/ui.

## Tech Stack

- **Vite** - Build tool and dev server
- **React 19** - UI framework
- **TypeScript** - Type safety
- **React Router** - Client-side routing
- **shadcn/ui** - UI component library (New York style, Zinc color)
- **Tailwind CSS** - Styling
- **Lucide React** - Icons

## Getting Started

### Install Dependencies

```bash
npm install
```

### Development

```bash
npm run dev
```

The application will be available at `http://localhost:5173`

### Build

```bash
npm run build
```

### Preview Production Build

```bash
npm run preview
```

## Project Structure

```
dashboard/
├── src/
│   ├── components/
│   │   ├── ui/           # shadcn/ui components
│   │   └── Layout.tsx    # Main layout with header
│   ├── pages/
│   │   ├── Landing.tsx       # Landing page with features
│   │   ├── Dashboard.tsx     # Dashboard with API key and usage
│   │   └── RequestQuota.tsx  # Quota request form
│   ├── lib/
│   │   ├── utils.ts      # Utility functions (cn)
│   │   └── api.ts        # API client stub
│   ├── hooks/
│   │   └── use-toast.ts  # Toast hook
│   ├── App.tsx           # Main app with routing
│   └── index.css         # Global styles and Tailwind
├── components.json       # shadcn/ui configuration
└── tailwind.config.js    # Tailwind configuration
```

## Routes

- `/` - Landing page with "Get API Key" button
- `/dashboard` - Dashboard showing API key and usage stats
- `/request-quota` - Form to request additional quota

## Features

### Landing Page
- Clean hero section with title and subtitle
- "Get API Key" CTA button
- Feature highlights grid

### Dashboard
- API key display with copy functionality
- Usage statistics (sessions today, quota remaining)
- Progress bar for quota usage
- Quick API example
- Link to request more quota

### Request Quota Form
- Fields: email, name, company (optional), use case, requested limits, budget (optional)
- Form validation
- Toast notification on submission
- Console logging of form data (ready for backend integration)

## API Integration

The API client stub is located at `src/lib/api.ts`. It includes:

- `ApiClient` class for authenticated requests
- `stubApiClient` with mock data for development
- Type definitions for API requests/responses

To integrate with the real backend:
1. Set `VITE_API_BASE_URL` environment variable
2. Replace stub implementations with real API calls
3. Add authentication flow

## Environment Variables

Create a `.env` file:

```
VITE_API_BASE_URL=http://localhost:8080
```

## Notes

- No authentication flow implemented yet - just UI structure
- All data is currently placeholder/stub data
- Form submission logs to console
- Ready for backend integration
