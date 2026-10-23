public final class HelloBarista {
    public static void main(String[] args) {
        StringBuilder sb = new StringBuilder();
        sb.append("Hello from Barista");
        if (args != null && args.length > 0) {
            sb.append(", ");
            sb.append(args[0]);
            sb.append("!");
        } else {
            sb.append("!");
        }
        System.out.println(sb.toString());
    }
}
