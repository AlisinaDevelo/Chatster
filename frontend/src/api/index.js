var socket = new WebSocket('ws://localhost:8080/ws');

let connect = (cb, setConnectionStatus) => {
  console.log("Attempting Connection..");
  
  if (setConnectionStatus) {
    setConnectionStatus('connecting');
  }

  socket.onopen = () => {
    console.log("Successfully Connected");
    if (setConnectionStatus) {
      setConnectionStatus('connected');
    }
  };

  socket.onmessage = (msg) => {
    console.log("Received message:", msg);
    cb(msg);
  };

  socket.onclose = (event) => {
    console.log("Socket Closed Connection: ", event);
    if (setConnectionStatus) {
      setConnectionStatus('disconnected');
    }
    
    // Try to reconnect after 2 seconds
    setTimeout(() => {
      socket = new WebSocket('ws://localhost:8080/ws');
      connect(cb, setConnectionStatus);
    }, 2000);
  };

  socket.onerror = (error) => {
    console.log("Socket Error: ", error);
    if (setConnectionStatus) {
      setConnectionStatus('error');
    }
  };
};

let sendMsg = (msg) => {
  console.log("sending msg: ", msg);
  if (socket.readyState === WebSocket.OPEN) {
    socket.send(msg);
  } else {
    console.log("Socket not connected, message not sent");
  }
}

export { connect, sendMsg };
