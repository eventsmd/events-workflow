package md.address.events.messaging;

import io.awspring.cloud.sqs.operations.SqsTemplate;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

@Service
public class MessageSender {

    @Value("${AWS_SQS_QUEUE_NAME}")
    private String queueName;

    private final SqsTemplate sqsTemplate;

    public MessageSender(SqsTemplate sqsTemplate) {
        this.sqsTemplate = sqsTemplate;
    }

    public void sendMessage(MessageToSend message) {
        sqsTemplate.send(to -> to.queue(queueName)
                .payload(message)
        );
    }
}
