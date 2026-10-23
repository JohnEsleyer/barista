import com.sun.net.httpserver.HttpServer;
import com.sun.net.httpserver.HttpHandler;
import com.sun.net.httpserver.HttpExchange;
import java.net.InetSocketAddress;
import java.net.URI;
import java.io.OutputStream;
import java.io.InputStream;
import java.io.IOException;

public class RestServer implements HttpHandler {
    private HttpServer server;

    public void start(int port) throws IOException {
        this.server = HttpServer.create(new InetSocketAddress(port), 0);
        this.server.createContext("/api/hello", this);
        this.server.createContext("/api/echo", this);
        this.server.start();
    }

    public int getPort() {
        if (this.server == null) {
            return -1;
        }
        InetSocketAddress addr = this.server.getAddress();
        if (addr == null) {
            return -1;
        }
        return addr.getPort();
    }

    public void stop() {
        if (this.server != null) {
            this.server.stop(0);
        }
    }

    public void handle(HttpExchange exchange) throws IOException {
        URI uri = exchange.getRequestURI();
        String path = uri.getPath();
        String method = exchange.getRequestMethod();

        if (path.equals("/api/hello")) {
            String response = "{\"status\":\"ok\",\"service\":\"barista\",\"method\":\"" + method + "\"}";
            byte[] bytes = response.getBytes();
            exchange.sendResponseHeaders(200, bytes.length);
            OutputStream os = exchange.getResponseBody();
            os.write(bytes);
            os.close();
        } else if (path.equals("/api/echo")) {
            InputStream is = exchange.getRequestBody();
            byte[] body = is.readAllBytes();
            exchange.sendResponseHeaders(200, body.length);
            OutputStream os = exchange.getResponseBody();
            os.write(body);
            os.close();
        } else {
            String notFound = "Not Found";
            byte[] bytes = notFound.getBytes();
            exchange.sendResponseHeaders(404, bytes.length);
            OutputStream os = exchange.getResponseBody();
            os.write(bytes);
            os.close();
        }
    }
}
