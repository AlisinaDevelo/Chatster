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

### Running the Application

#### Backend

1. Navigate to the backend directory:
   ```
   cd backend
   ```

2. Run the Go server:
   ```
   go run main.go
   ```

The backend server will start on http://localhost:8080.

#### Frontend

1. Navigate to the frontend directory:
   ```
   cd frontend
   ```

2. Install dependencies:
   ```
   npm install
   ```

3. Start the React development server:
   ```
   npm start
   ```

The frontend application will start on http://localhost:3000.

## Usage

1. Open your browser and navigate to http://localhost:3000
2. Enter your username when prompted
3. Start chatting with other connected users in real-time

## Future Enhancements

- Add user authentication with JWT
- Implement private messaging
- Add message persistence with SQLite
- Create chat rooms/channels

