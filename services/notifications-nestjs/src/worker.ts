import { NestFactory } from '@nestjs/core';

import { AppModule } from './app.module';
import { RabbitMqWorker } from './messaging/rabbitmq-worker';

async function bootstrap(): Promise<void> {
  const app = await NestFactory.createApplicationContext(AppModule);
  await app.get(RabbitMqWorker).start();
}

void bootstrap();
