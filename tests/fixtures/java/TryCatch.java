public final class TryCatch {
    private static int result;

    public static int run(boolean fail) {
        try {
            if (fail) {
                throw new RuntimeException("boom");
            }
            result = 1;
        } catch (RuntimeException e) {
            System.out.println(e.getMessage());
            result = 2;
        }
        return result;
    }
}
