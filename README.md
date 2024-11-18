# Chatster

Chatster is a real-time chat application built with Go (Golang) for the backend and React for the frontend. The application allows users to create accounts, log in, and chat with others in real-time.

## Features

- **Real-Time Messaging**: Chat with others instantly, with messages delivered in real-time.
- **User Authentication**: Secure user authentication and session management.
- **Responsive UI**: A responsive and modern user interface built with React.
- **WebSocket Integration**: Efficient communication between the client and server using WebSockets.

## Technologies Used

- **Backend**:
  - [Go](https://golang.org/): The backend server is built using Go, providing high performance and concurrency handling.
  - [Gorilla WebSocket](https://github.com/gorilla/websocket): Used for real-time communication between the server and clients.
  - [Gorilla Mux](https://github.com/gorilla/mux): HTTP request router and dispatcher for matching incoming requests to their respective handler.
  
- **Frontend**:
  - [React](https://reactjs.org/): A JavaScript library for building user interfaces.
  - [Axios](https://axios-http.com/): Used for making HTTP requests from the frontend to the backend.
  
- **Database**:
  - [SQLite](https://www.sqlite.org/): A lightweight, disk-based database used to store user data and messages.

## Getting Started

### Prerequisites

- Go (1.16+)
- Node.js (14.x or later)
- npm (6.x or later)

