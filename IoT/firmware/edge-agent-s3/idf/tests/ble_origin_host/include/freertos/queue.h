#pragma once
#include "FreeRTOS.h"
struct FakeQueue;using QueueHandle_t=FakeQueue*;
QueueHandle_t xQueueCreate(size_t,size_t);
int xQueueSend(QueueHandle_t,const void*,int);
int xQueueReceive(QueueHandle_t,void*,int);
