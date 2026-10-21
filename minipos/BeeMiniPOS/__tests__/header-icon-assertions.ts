import React from 'react';
// Paper accepts either an image source or an icon renderer. Assert the asset
// and renderer accessibility contract, rather than one internal prop shape.
export function expectHeaderAsset(icon:unknown,expected:unknown){
 if(typeof icon!=='function'){expect(icon).toBe(expected);return;}
 const rendered=icon({size:20,color:'#123456'}) as React.ReactElement<{children:React.ReactNode}>;
 expect(React.isValidElement(rendered)).toBe(true);
 expect(rendered.props).toMatchObject({'aria-hidden':true,accessibilityElementsHidden:true,importantForAccessibility:'no-hide-descendants'});
 const child=React.Children.only(rendered.props.children) as React.ReactElement<{source:unknown;size:number;color:string}>;
 expect(child.type).toBe('Paper.Icon');
 expect(child.props).toMatchObject({source:expected,size:20,color:'#123456'});
}
