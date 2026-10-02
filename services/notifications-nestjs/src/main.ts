import { NestFactory } from '@nestjs/core';
import { ConfigService } from '@nestjs/config';

import { AppModule } from './app.module';
import { configureApp } from './configure-app';
import { RabbitMqWorker } from './messaging/rabbitmq-worker';

async function bootstrap(): Promise<void> {
  const app = await NestFactory.create(AppModule);
  configureApp(app);

  const configService = app.get(ConfigService);
  const port = Number(configService.get<string>('HTTP_PORT', '3000'));

  await app.listen(port);
  await app.get(RabbitMqWorker).start();
}

void bootstrap();
