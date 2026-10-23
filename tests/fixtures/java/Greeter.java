public final class Greeter {
    public static int run() {
        String message = "Freshly brewed with Barista!";
        System.out.println(message);

        StringBuilder builder = new StringBuilder();
        builder.append("Barista #");
        builder.append(7);
        System.out.println(builder.toString());

        int[] numbers = new int[3];
        numbers[0] = 42;
        numbers[1] = 99;

        String[] labels = new String[2];
        labels[0] = message;
        labels[1] = builder.toString();

        Class<?> type = String.class;

        return numbers[0] + numbers[1] + numbers.length
                + message.length() + builder.toString().length()
                + labels[0].length() + labels[1].length()
                + type.getName().length();
    }
}
