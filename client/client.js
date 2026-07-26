import WebSocket from "ws";

const socket = new WebSocket("ws://localhost:7007/ws");

socket.on("open", () => {
    console.log("Connected to server");
    socket.send("Hello");
});

socket.on("message", (data) => {
    console.log("Received from server:", data.toString());
});

socket.on("error", (err) => {
    console.error("WebSocket error:", err);
});

socket.on("close", () => {
    console.log("Connection closed");
});