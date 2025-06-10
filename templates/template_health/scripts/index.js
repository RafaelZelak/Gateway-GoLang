// Clock initialization
function startClock() {
  const clock = document.getElementById("clock");
  const textNode = document.createTextNode("Conectando...");
  clock.textContent = "";
  clock.appendChild(textNode);

  const socket = new WebSocket("ws://localhost:8080/clock");

  socket.onopen = () => {
    textNode.nodeValue = "Conectado...";
  };

  socket.onmessage = (event) => {
    const data = JSON.parse(event.data);
    textNode.nodeValue = data.time;
  };

  socket.onerror = () => {
    textNode.nodeValue = "Erro na conexão";
  };

  socket.onclose = () => {
    textNode.nodeValue = "Reconectando...";
    setTimeout(startClock, 1000);
  };
}

startClock();
