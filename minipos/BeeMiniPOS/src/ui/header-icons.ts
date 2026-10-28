import React from 'react';
import {View} from 'react-native';
import {Icon} from 'react-native-paper';
// Exact MaterialCommunityIcons images reused from Company POS; see header asset provenance.
export const headerIcons={
  'arrow-left':require('../../assets/header-icons/arrow-left.png'),
  'menu':require('../../assets/header-icons/menu.png'),
  'translate':require('../../assets/header-icons/translate.png'),
  'weather-night':require('../../assets/header-icons/weather-night.png'),
  'weather-sunny':require('../../assets/header-icons/weather-sunny.png'),
} as const;

/** Buttons own the accessible name; their image is decorative. Keep genuine Paper Icon. */
export const decorativeHeaderIcon=(name:keyof typeof headerIcons)=>({size,color}:{size:number;color:string})=>
 React.createElement(View,{'aria-hidden':true,accessibilityElementsHidden:true,importantForAccessibility:'no-hide-descendants'},
  React.createElement(Icon,{source:headerIcons[name],size,color}));
