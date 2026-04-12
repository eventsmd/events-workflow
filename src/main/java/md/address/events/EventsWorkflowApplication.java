package md.address.events;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.annotation.ImportRuntimeHints;

@SpringBootApplication
@ImportRuntimeHints(NativeImageHints.class)
public class EventsWorkflowApplication {

    public static void main(String[] args) {
        SpringApplication.run(EventsWorkflowApplication.class, args);
    }
}
