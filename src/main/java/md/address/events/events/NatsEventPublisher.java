package md.address.events.events;

import com.fasterxml.jackson.databind.ObjectMapper;
import io.nats.client.Connection;
import io.nats.client.JetStream;
import io.nats.client.JetStreamManagement;
import io.nats.client.Nats;
import io.nats.client.Options;
import io.nats.client.PublishOptions;
import io.nats.client.api.StorageType;
import io.nats.client.api.StreamConfiguration;
import io.nats.client.impl.NatsMessage;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.time.Duration;

@Service
public class NatsEventPublisher {

    private static final Logger log = LoggerFactory.getLogger(NatsEventPublisher.class);

    private final ObjectMapper objectMapper;
    private final String natsUrl;
    private final String natsUser;
    private final String natsPass;
    private final String natsCreds;
    private final String stream;
    private final String subjectPrefix;
    private final Duration streamMaxAge;

    private volatile Connection connection;
    private volatile JetStream jetStream;

    public NatsEventPublisher(
            ObjectMapper objectMapper,
            @Value("${NATS_URL:}") String natsUrl,
            @Value("${NATS_USER:}") String natsUser,
            @Value("${NATS_PASS:}") String natsPass,
            @Value("${NATS_CREDS:}") String natsCreds,
            @Value("${NATS_STREAM:UTILITY}") String stream,
            @Value("${NATS_SUBJECT_PREFIX:pmr.utility.event}") String subjectPrefix,
            @Value("${NATS_STREAM_MAX_AGE:PT24H}") Duration streamMaxAge) {
        this.objectMapper = objectMapper;
        this.natsUrl = natsUrl;
        this.natsUser = natsUser;
        this.natsPass = natsPass;
        this.natsCreds = natsCreds;
        this.stream = stream;
        this.subjectPrefix = subjectPrefix;
        this.streamMaxAge = streamMaxAge;
    }

    public void publish(UtilityEvent event) {
        if (natsUrl == null || natsUrl.isBlank()) {
            log.debug("NATS_URL not set — skipping event publish for incident {}", event.incidentId());
            return;
        }
        try {
            JetStream js = ensureConnected();
            String subject = subjectFor(subjectPrefix, event.supplier(), event.event());
            byte[] data = objectMapper.writeValueAsBytes(event);
            var msg = NatsMessage.builder()
                    .subject(subject)
                    .data(data)
                    .build();
            js.publish(msg, PublishOptions.builder().messageId(event.incidentId()).build());
            log.info("Published utility event {} to {}", event.incidentId(), subject);
        } catch (Exception e) {
            log.error("Failed to publish utility event {} to NATS", event.incidentId(), e);
        }
    }

    private synchronized JetStream ensureConnected() throws Exception {
        if (jetStream != null && connection != null
                && connection.getStatus() == Connection.Status.CONNECTED) {
            return jetStream;
        }
        Options.Builder opts = new Options.Builder().server(natsUrl).connectionTimeout(Duration.ofSeconds(5));
        if (natsCreds != null && !natsCreds.isBlank()) {
            opts.authHandler(Nats.credentials(natsCreds));
        } else if (natsUser != null && !natsUser.isBlank()) {
            opts.userInfo(natsUser, natsPass);
        }
        Connection conn = Nats.connect(opts.build());
        try {
            JetStreamManagement jsm = conn.jetStreamManagement();
            StreamConfiguration sc = StreamConfiguration.builder()
                    .name(stream)
                    .subjects(subjectPrefix + ".>")
                    .storageType(StorageType.File)
                    .maxAge(streamMaxAge)
                    .build();
            try {
                jsm.addStream(sc);
            } catch (Exception alreadyExists) {
                jsm.updateStream(sc);
            }
            connection = conn;
            jetStream = conn.jetStream();
            return jetStream;
        } catch (Exception setupFailure) {
            connection = null;
            jetStream = null;
            try {
                conn.close();
            } catch (Exception closeFailure) {
                log.warn("Failed to close NATS connection after setup failure", closeFailure);
            }
            throw setupFailure;
        }
    }

    public static String subjectFor(String prefix, String supplier, String event) {
        return prefix + "." + sanitizeToken(supplier) + "." + sanitizeToken(event);
    }

    public static String sanitizeToken(String s) {
        if (s == null || s.isEmpty()) {
            return "_";
        }
        StringBuilder b = new StringBuilder();
        boolean prevUnderscore = false;
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            if (c == ' ' || c == '\t' || c == '\n' || c == '\r'
                    || c == '.' || c == '*' || c == '>' || c == '/') {
                if (!prevUnderscore) {
                    b.append('_');
                    prevUnderscore = true;
                }
            } else {
                b.append(c);
                prevUnderscore = false;
            }
        }
        String out = b.toString();
        int start = 0, end = out.length();
        while (start < end && out.charAt(start) == '_') start++;
        while (end > start && out.charAt(end - 1) == '_') end--;
        out = out.substring(start, end);
        return out.isEmpty() ? "_" : out;
    }
}
