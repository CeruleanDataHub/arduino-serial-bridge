
/*  
 *  4-20 mA Sensor Board
 *  
 *  Copyright (C) Libelium Comunicaciones Distribuidas S.L. 
 *  http://www.libelium.com 
 *  
 *  This program is free software: you can redistribute it and/or modify 
 *  it under the terms of the GNU General Public License as published by 
 *  the Free Software Foundation, either version 3 of the License, or 
 *  (at your option) any later version. 
 *  a
 *  This program is distributed in the hope that it will be useful, 
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of 
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the 
 *  GNU General Public License for more details.
 *  
 *  You should have received a copy of the GNU General Public License 
 *  along with this program.  If not, see http://www.gnu.org/licenses/. 
 *  
 *  Version:           1.0
 *  Design:            David Gascón 
 *  Implementation:    Ahmad Saad
 */

// Include this library for using current loop functions.
#include <currentLoop.h >

#define CHANNEL CHANNEL1

void setup()
{
  
  // Switch ON the 24V DC-DC converter
  sensorBoard.ON();

  // Inits the Serial for viewing data in the serial monitor
  Serial.begin(115200);
  delay(100);
}


void loop()
{
  String output = "";
  // Get the sensor value in int format (0-1023)
  int value = sensorBoard.readChannel(CHANNEL);
  output = value;
  // Get the sensor value as a voltage in Volts
  float voltage = sensorBoard.readVoltage(CHANNEL);
  output = output + '|' + voltage;
  // Get the sensor value as a curren in mA
  float current = sensorBoard.readCurrent(CHANNEL);
  output = output + '|' + current;
    
  Serial.println(output);

  delay(2000);
}
    
