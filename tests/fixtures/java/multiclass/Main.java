public class Main {
    public static void main(String[] args) {
        GreeterService service = new GreeterService("Welcome to Barista");
        String name = args.length > 0 ? args[0] : "Native Go";
        System.out.println(service.prefix);
        System.out.println(service.format(name));
    }
}
