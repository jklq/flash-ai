# FlashAI

FlashAI is a modern, intelligent flashcard application built with React, TypeScript, and Vite. It leverages the `ts-fsrs` spaced repetition library to optimize learning and retention.

![App Screenshot](public/app-screenshot.png)

## Features

-   **Smart Sprints**: Review flashcards in focused sprints (default 10 cards) to maintain engagement.
-   **Spaced Repetition**: Utilizes the [FSRS](https://github.com/open-spaced-repetition/ts-fsrs) algorithm to schedule reviews efficiently.
-   **Topic-Based Learning**: Organize study sessions by topics such as Processes, Memory, Concurrency, and Storage.
-   **Instant Feedback**: Visual feedback on card flips and answers.
-   **Progress Tracking**: Overview page ranking topics by performance and highlighting difficult questions.
-   **Gamification**: Celebratory confetti effects upon sprint completion!

## Tech Stack

-   **Framework**: [React](https://react.dev/) + [Vite](https://vitejs.dev/)
-   **Language**: [TypeScript](https://www.typescriptlang.org/)
-   **Styling**: [Tailwind CSS](https://tailwindcss.com/)
-   **Algorithm**: [ts-fsrs](https://github.com/open-spaced-repetition/ts-fsrs)
-   **Icons**: [Lucide React](https://lucide.dev/)

## Getting Started

### Prerequisites

-   Node.js (v18 or higher)
-   pnpm (recommended) or npm/yarn

### Installation

1.  Clone the repository:
    ```bash
    git clone <repository-url>
    cd flash-ai
    ```

2.  Install dependencies:
    ```bash
    pnpm install
    # or
    npm install
    ```

### Running Locally

Start the development server:

```bash
pnpm run dev
# or
npm run dev
```

The application will be available at `http://localhost:5173/`.

## Building for Production

To build the application for production:

```bash
pnpm run build
```

## License

MIT
