# LogiLens Frontend

## Quick Start

### Option 1: Python HTTP Server (Recommended)

```bash
# Navigate to the frontend directory
cd frontend

# Run the server
python3 server.py
```

The server will start on `http://localhost:8000` and automatically open your browser.

### Option 2: Using Python's Built-in Server

```bash
cd frontend
python3 -m http.server 8000
```

Then open `http://localhost:8000` in your browser.

### Option 3: Using Node.js (if you have it installed)

```bash
cd frontend
npx http-server -p 8000
```

Then open `http://localhost:8000` in your browser.

## Why a Server is Needed

Modern browsers block `fetch()` requests when opening HTML files directly from the file system (`file://` protocol) due to CORS (Cross-Origin Resource Sharing) security policies. A local web server is required to serve the files over HTTP, which allows the JavaScript to load pages and components correctly.

## Project Structure

```
frontend/
├── index.html          # Main entry point
├── styles.css          # Custom CSS variables and styles
├── app.js              # Navigation and screen management
├── server.py           # Local development server
├── components/
│   ├── sidebar.html    # Reusable sidebar component
│   └── topbar.html     # Reusable topbar component
└── pages/
    ├── landing.html
    ├── login.html
    ├── register.html
    ├── dashboard.html
    ├── create-shipment.html
    ├── shipment-details.html
    ├── network-map.html
    ├── analytics.html
    └── admin.html
```

## Features

- **Modular Structure**: Each page is a separate HTML file
- **Reusable Components**: Sidebar and topbar are shared components
- **Tailwind CSS**: Modern utility-first CSS framework
- **Dynamic Navigation**: JavaScript handles screen switching
- **Design Preserved**: Original styling maintained with CSS variables
