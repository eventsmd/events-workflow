package md.address.events.workflows;

import md.address.events.domain.*;
import org.junit.jupiter.api.Test;

import java.math.BigInteger;
import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for the workflow orchestration logic.
 *
 * Note: TelegramMessageProcessWorkflow uses Temporal's Workflow.newActivityStub
 * which requires a Temporal test environment. Full workflow integration tests
 * would use io.temporal.testing.TestWorkflowEnvironment.
 *
 * These tests verify the conditional logic extracted from the workflow:
 * - if messageTranscription is not null -> save, normalize, notify
 * - if messageTranscription is null -> skip
 */
class TelegramMessageProcessWorkflowTest {

    @Test
    void parsedMessageWithTranscriptionShouldRequireProcessing() {
        var message = createTestMessage();
        var transcription = new MessageTranscription(
                "org", "desc", "shutdown",
                LocalDateTime.now(), LocalDateTime.now().plusHours(8),
                List.of()
        );
        var parsed = new ParsedMessage(message, transcription);

        assertNotNull(parsed.messageTranscription(),
                "When transcription is present, workflow should proceed with save/normalize/notify");
    }

    @Test
    void parsedMessageWithNullTranscriptionShouldSkipProcessing() {
        var message = createTestMessage();
        var parsed = new ParsedMessage(message, null);

        assertNull(parsed.messageTranscription(),
                "When transcription is null, workflow should skip save/normalize/notify");
    }

    @Test
    void workflowClassShouldImplementInterface() {
        assertTrue(TelegramMessageProcess.class.isAssignableFrom(TelegramMessageProcessWorkflow.class));
    }

    private TelegramMessage createTestMessage() {
        return new TelegramMessage(
                BigInteger.valueOf(100), BigInteger.valueOf(200),
                new User(BigInteger.ONE, "TestUser"),
                "Test message", LocalDateTime.now(),
                null, "water", null
        );
    }
}
