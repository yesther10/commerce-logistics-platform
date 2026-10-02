import { IsNotEmpty, IsString, MaxLength } from 'class-validator';

export class CreateNotificationRequestDto {
  @IsString()
  @IsNotEmpty()
  @MaxLength(255)
  order_id!: string;

  @IsString()
  @IsNotEmpty()
  @MaxLength(255)
  shipment_id!: string;

  @IsString()
  @IsNotEmpty()
  @MaxLength(100)
  notification_type!: string;
}
