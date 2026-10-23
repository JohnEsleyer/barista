public class GreeterService {
    public String prefix;

    public GreeterService(String prefix) {
        this.prefix = prefix;
    }

    public String format(String name) {
        StringBuilder sb = new StringBuilder();
        sb.append(this.prefix);
        sb.append(", ");
        sb.append(name);
        sb.append("!");
        return sb.toString();
    }
}
