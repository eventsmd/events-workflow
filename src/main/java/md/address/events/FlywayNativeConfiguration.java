package md.address.events;

import org.springframework.boot.autoconfigure.flyway.FlywayConfigurationCustomizer;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.NativeDetector;
import org.springframework.core.io.ClassPathResource;

import java.io.InputStreamReader;
import java.io.Reader;
import java.nio.charset.StandardCharsets;
import java.util.Collection;
import java.util.List;

@Configuration
public class FlywayNativeConfiguration {

    private static final List<String> MIGRATION_FILES = List.of(
            "db/migration/V202512012250__create_telegram_messages.sql",
            "db/migration/V202512030730__create_subscribtions_table.sql"
    );

    @Bean
    FlywayConfigurationCustomizer nativeImageFlywayCustomizer() {
        return configuration -> {
            if (NativeDetector.inNativeImage()) {
                configuration.resourceProvider(new NativeFlywayResourceProvider());
            }
        };
    }

    static class NativeFlywayResourceProvider implements org.flywaydb.core.api.ResourceProvider {

        @Override
        public org.flywaydb.core.api.resource.LoadableResource getResource(String name) {
            for (String file : MIGRATION_FILES) {
                if (file.endsWith(name) || file.equals(name)) {
                    return toLoadableResource(file);
                }
            }
            return null;
        }

        @Override
        public Collection<org.flywaydb.core.api.resource.LoadableResource> getResources(
                String prefix, String[] suffixes) {
            return MIGRATION_FILES.stream()
                    .filter(f -> {
                        for (String suffix : suffixes) {
                            if (f.endsWith(suffix)) return true;
                        }
                        return false;
                    })
                    .map(NativeFlywayResourceProvider::toLoadableResource)
                    .toList();
        }

        private static org.flywaydb.core.api.resource.LoadableResource toLoadableResource(String path) {
            return new org.flywaydb.core.api.resource.LoadableResource() {
                @Override
                public Reader read() {
                    var resource = new ClassPathResource(path);
                    try {
                        return new InputStreamReader(resource.getInputStream(), StandardCharsets.UTF_8);
                    } catch (Exception e) {
                        throw new RuntimeException("Cannot read migration: " + path, e);
                    }
                }

                @Override
                public String getAbsolutePath() {
                    return path;
                }

                @Override
                public String getAbsolutePathOnDisk() {
                    return path;
                }

                @Override
                public String getRelativePath() {
                    return path;
                }

                @Override
                public String getFilename() {
                    int idx = path.lastIndexOf('/');
                    return idx >= 0 ? path.substring(idx + 1) : path;
                }
            };
        }
    }
}
